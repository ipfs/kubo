//go:build (linux || darwin || freebsd) && !nofuse

// Unit tests for the mutable /ipns FUSE mount.
// These test the filesystem implementation directly without a daemon.
// End-to-end tests that exercise mount/unmount through a real daemon
// live in test/cli/fuse/.

package ipns

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	mrand "math/rand"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/stretchr/testify/require"

	mfs "github.com/ipfs/boxo/mfs"
	racedet "github.com/ipfs/go-detect-race"
	"github.com/ipfs/go-test/random"
	"github.com/ipfs/kubo/config"
	core "github.com/ipfs/kubo/core"
	coreapi "github.com/ipfs/kubo/core/coreapi"
	iface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipfs/kubo/fuse/fusetest"
	fusemnt "github.com/ipfs/kubo/fuse/mount"
)

func randBytes(size int) []byte {
	b := make([]byte, size)
	_, err := io.ReadFull(random.NewRand(), b)
	if err != nil {
		panic(err)
	}
	return b
}

func mkdir(t *testing.T, path string) {
	err := os.Mkdir(path, os.ModeDir)
	if err != nil {
		t.Fatal(err)
	}
}

func writeFileOrFail(t *testing.T, size int, path string) []byte {
	data, err := writeFile(size, path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeFile(size int, path string) ([]byte, error) {
	data := randBytes(size)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return nil, err
	}
	_, err = f.Write(data)
	if err != nil {
		f.Close()
		return nil, err
	}
	// Go's goroutine preemption (SIGURG) can interrupt the FUSE FLUSH
	// inside close(), returning EINTR. This is not a data loss: the write
	// already succeeded and the kernel will still send RELEASE to the FUSE
	// daemon.
	if err := f.Close(); err != nil && !errors.Is(err, syscall.EINTR) {
		return nil, err
	}
	return data, nil
}

func verifyFile(t *testing.T, path string, wantData []byte) {
	isData, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(isData) != len(wantData) {
		t.Fatal("Data not equal - length check failed")
	}
	if !bytes.Equal(isData, wantData) {
		t.Fatal("Data not equal")
	}
}

func checkExists(t *testing.T, path string) {
	_, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
}

type mountWrap struct {
	Dir    string
	Root   *Root
	server *fuse.Server
	closed bool
}

func (m *mountWrap) Close() {
	if m.closed {
		return
	}
	m.closed = true
	if m.server != nil {
		_ = m.server.Unmount()
	}
	_ = m.Root.Close()
}

func closeMount(mnt *mountWrap) {
	if err := recover(); err != nil {
		log.Error("Recovered panic")
		log.Error(err)
	}
	mnt.Close()
}

// fakeMount is a minimal mount.Mount that reports itself as active.
// This simulates the real daemon path where node.Mounts.Ipns is set
// after the FUSE filesystem is mounted, ensuring that checkPublishAllowed
// is actually exercised during tests (see issue #2168).
type fakeMount struct{}

func (fakeMount) MountPoint() string { return "/fake/ipns" }
func (fakeMount) Unmount() error     { return nil }
func (fakeMount) IsActive() bool     { return true }

func setupIpnsTest(t *testing.T, node *core.IpfsNode, cfgs ...config.Mounts) (*core.IpfsNode, *mountWrap) {
	t.Helper()
	fusetest.SkipUnlessFUSE(t)

	var cfg config.Mounts
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}

	var err error
	if node == nil {
		node, err = core.NewNode(context.Background(), &core.BuildCfg{})
		if err != nil {
			t.Fatal(err)
		}

		err = InitializeKeyspace(node, node.PrivateKey)
		if err != nil {
			t.Fatal(err)
		}
	}

	coreAPI, err := coreapi.NewCoreAPI(node)
	if err != nil {
		t.Fatal(err)
	}

	key, err := coreAPI.Key().Self(node.Context())
	if err != nil {
		t.Fatal(err)
	}

	root, err := CreateRoot(node.Context(), coreAPI, map[string]iface.Key{"local": key}, "", "", cfg)
	if err != nil {
		t.Fatal(err)
	}

	mntDir := t.TempDir()
	server, err := fs.Mount(mntDir, root, &fs.Options{
		NullPermissions: true,
		UID:             uint32(os.Getuid()),
		GID:             uint32(os.Getgid()),
		EntryTimeout:    &mutableCacheTime,
		AttrTimeout:     &mutableCacheTime,
		MountOptions: fuse.MountOptions{
			FsName:            "kubo-test",
			MaxReadAhead:      fusemnt.MaxReadAhead,
			ExtraCapabilities: fusemnt.WritableMountCapabilities,
		},
	})
	fusetest.MountError(t, err)

	mnt := &mountWrap{Dir: mntDir, Root: root, server: server}
	t.Cleanup(mnt.Close)

	// Simulate the real daemon: set node.Mounts.Ipns so that
	// checkPublishAllowed sees an active IPNS mount.
	node.Mounts.Ipns = fakeMount{}

	return node, mnt
}

func TestIpnsLocalLink(t *testing.T) {
	nd, mnt := setupIpnsTest(t, nil)
	name := mnt.Dir + "/local"

	checkExists(t, name)

	linksto, err := os.Readlink(name)
	if err != nil {
		t.Fatal(err)
	}

	if linksto != nd.Identity.String() {
		t.Fatal("Link invalid")
	}
}

// Test that empty directories can be listed without errors.
func TestEmptyDirListing(t *testing.T) {
	nd, mnt := setupIpnsTest(t, nil)

	// The peer's IPNS directory starts empty.
	peerDir := mnt.Dir + "/" + nd.Identity.String()
	entries, err := os.ReadDir(peerDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty peer dir, got %d entries", len(entries))
	}

	// Create a subdirectory and list it while still empty.
	subdir := peerDir + "/emptydir"
	if err := os.Mkdir(subdir, os.ModeDir); err != nil {
		t.Fatal(err)
	}
	entries, err = os.ReadDir(subdir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty subdirectory, got %d entries", len(entries))
	}
}

// Test writing a file and reading it back.
func TestIpnsBasicIO(t *testing.T) {
	nd, mnt := setupIpnsTest(t, nil)
	defer closeMount(mnt)

	fname := mnt.Dir + "/local/testfile"
	data := writeFileOrFail(t, 10, fname)

	rbuf, err := os.ReadFile(fname)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(rbuf, data) {
		t.Fatal("Incorrect Read!")
	}

	fname2 := mnt.Dir + "/" + nd.Identity.String() + "/testfile"
	rbuf, err = os.ReadFile(fname2)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(rbuf, data) {
		t.Fatal("Incorrect Read!")
	}
}

// Test renaming a file within the same IPNS directory.
func TestRenameFile(t *testing.T) {
	nd, mnt := setupIpnsTest(t, nil)
	defer closeMount(mnt)

	peerDir := mnt.Dir + "/" + nd.Identity.String()
	src := peerDir + "/before.txt"
	dst := peerDir + "/after.txt"

	data := writeFileOrFail(t, 500, src)

	if err := os.Rename(src, dst); err != nil {
		t.Fatal(err)
	}

	// Source must be gone.
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source still exists after rename: %v", err)
	}

	// Destination must have the original content.
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("content mismatch: got %d bytes, want %d", len(got), len(data))
	}
}

// Test removing a file.
func TestRemoveFile(t *testing.T) {
	_, mnt := setupIpnsTest(t, nil)
	defer closeMount(mnt)

	fname := mnt.Dir + "/local/removeme"
	writeFileOrFail(t, 200, fname)
	checkExists(t, fname)

	if err := os.Remove(fname); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(fname); !os.IsNotExist(err) {
		t.Fatalf("file still exists after remove: %v", err)
	}
}

// Test that removing a non-empty directory fails.
func TestRemoveNonEmptyDirectory(t *testing.T) {
	_, mnt := setupIpnsTest(t, nil)
	defer closeMount(mnt)

	dir := mnt.Dir + "/local/mydir"
	mkdir(t, dir)
	writeFileOrFail(t, 100, dir+"/child")

	// Removing a non-empty directory must fail.
	err := syscall.Rmdir(dir)
	if err == nil {
		t.Fatal("expected error removing non-empty directory")
	}

	// The directory and its child must still exist.
	checkExists(t, dir)
	checkExists(t, dir+"/child")

	// After removing the child, the directory can be removed.
	if err := os.Remove(dir + "/child"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("directory still exists after remove: %v", err)
	}
}

// Test to make sure file changes persist over mounts of ipns.
func TestFilePersistence(t *testing.T) {
	node, mnt := setupIpnsTest(t, nil)

	fname := "/local/atestfile"
	data := writeFileOrFail(t, 127, mnt.Dir+fname)

	mnt.Close() // flush MFS roots before remounting

	// Remount with the same node.
	_, mnt = setupIpnsTest(t, node)

	rbuf, err := os.ReadFile(mnt.Dir + fname)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(rbuf, data) {
		t.Fatalf("File data changed between mounts! sizes differ: %d != %d", len(data), len(rbuf))
	}
}

func TestMultipleDirs(t *testing.T) {
	node, mnt := setupIpnsTest(t, nil)

	t.Log("make a top level dir")
	dir1 := "/local/test1"
	mkdir(t, mnt.Dir+dir1)

	checkExists(t, mnt.Dir+dir1)

	t.Log("write a file in it")
	data1 := writeFileOrFail(t, 4000, mnt.Dir+dir1+"/file1")

	verifyFile(t, mnt.Dir+dir1+"/file1", data1)

	t.Log("sub directory")
	mkdir(t, mnt.Dir+dir1+"/dir2")

	checkExists(t, mnt.Dir+dir1+"/dir2")

	t.Log("file in that subdirectory")
	data2 := writeFileOrFail(t, 5000, mnt.Dir+dir1+"/dir2/file2")

	verifyFile(t, mnt.Dir+dir1+"/dir2/file2", data2)

	t.Log("closing mount, then restarting")
	mnt.Close() // flush MFS roots before remounting

	_, mnt = setupIpnsTest(t, node)

	checkExists(t, mnt.Dir+dir1)

	verifyFile(t, mnt.Dir+dir1+"/file1", data1)

	verifyFile(t, mnt.Dir+dir1+"/dir2/file2", data2)
}

// Test to make sure the filesystem reports file sizes correctly.
func TestFileSizeReporting(t *testing.T) {
	_, mnt := setupIpnsTest(t, nil)

	fname := mnt.Dir + "/local/sizecheck"
	data := writeFileOrFail(t, 5555, fname)

	finfo, err := os.Stat(fname)
	if err != nil {
		t.Fatal(err)
	}

	if finfo.Size() != int64(len(data)) {
		t.Fatal("Read incorrect size from stat!")
	}
}

// Test to make sure you can't create multiple entries with the same name.
func TestDoubleEntryFailure(t *testing.T) {
	_, mnt := setupIpnsTest(t, nil)

	dname := mnt.Dir + "/local/thisisadir"
	err := os.Mkdir(dname, 0o777)
	if err != nil {
		t.Fatal(err)
	}

	err = os.Mkdir(dname, 0o777)
	if err == nil {
		t.Fatal("Should have gotten error one creating new directory.")
	}
}

func TestAppendFile(t *testing.T) {
	_, mnt := setupIpnsTest(t, nil)

	fname := mnt.Dir + "/local/file"
	data := writeFileOrFail(t, 1300, fname)

	fi, err := os.OpenFile(fname, os.O_RDWR|os.O_APPEND, 0o666)
	if err != nil {
		t.Fatal(err)
	}

	nudata := randBytes(500)

	n, err := fi.Write(nudata)
	if err != nil {
		t.Fatal(err)
	}
	err = fi.Close()
	if err != nil {
		t.Fatal(err)
	}

	if n != len(nudata) {
		t.Fatal("Failed to write enough bytes.")
	}

	data = append(data, nudata...)

	rbuf, err := os.ReadFile(fname)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rbuf, data) {
		t.Fatal("Data inconsistent!")
	}
}

func TestConcurrentWrites(t *testing.T) {
	_, mnt := setupIpnsTest(t, nil)

	nactors := 4
	filesPerActor := 400
	fileSize := 2000

	data := make([][][]byte, nactors)

	if racedet.WithRace() {
		nactors = 2
		filesPerActor = 50
	}

	wg := sync.WaitGroup{}
	for i := 0; i < nactors; i++ {
		data[i] = make([][]byte, filesPerActor)
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < filesPerActor; j++ {
				out, err := writeFile(fileSize, mnt.Dir+fmt.Sprintf("/local/%dFILE%d", n, j))
				if err != nil {
					t.Error(err)
					continue
				}
				data[n][j] = out
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < nactors; i++ {
		for j := 0; j < filesPerActor; j++ {
			if data[i][j] == nil {
				// Error already reported.
				continue
			}
			verifyFile(t, mnt.Dir+fmt.Sprintf("/local/%dFILE%d", i, j), data[i][j])
		}
	}
}

func TestFSThrash(t *testing.T) {
	files := make(map[string][]byte)

	_, mnt := setupIpnsTest(t, nil)

	base := mnt.Dir + "/local"
	dirs := []string{base}
	dirlock := sync.RWMutex{}
	filelock := sync.Mutex{}

	ndirWorkers := 2
	nfileWorkers := 2

	ndirs := 100
	nfiles := 200

	wg := sync.WaitGroup{}

	// Spawn off workers to make directories
	for i := range ndirWorkers {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := range ndirs {
				dirlock.RLock()
				n := mrand.Intn(len(dirs))
				dir := dirs[n]
				dirlock.RUnlock()

				newDir := fmt.Sprintf("%s/dir%d-%d", dir, worker, j)
				err := os.Mkdir(newDir, os.ModeDir)
				if err != nil {
					t.Error(err)
					continue
				}
				dirlock.Lock()
				dirs = append(dirs, newDir)
				dirlock.Unlock()
			}
		}(i)
	}

	// Spawn off workers to make files
	for i := range nfileWorkers {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := range nfiles {
				dirlock.RLock()
				n := mrand.Intn(len(dirs))
				dir := dirs[n]
				dirlock.RUnlock()

				newFileName := fmt.Sprintf("%s/file%d-%d", dir, worker, j)

				data, err := writeFile(2000+mrand.Intn(5000), newFileName)
				if err != nil {
					t.Error(err)
					continue
				}
				filelock.Lock()
				files[newFileName] = data
				filelock.Unlock()
			}
		}(i)
	}

	wg.Wait()
	for name, data := range files {
		out, err := os.ReadFile(name)
		if err != nil {
			t.Error(err)
		}

		if !bytes.Equal(data, out) {
			t.Errorf("Data didn't match in %s: expected %v, got %v", name, data, out)
		}
	}
}

// Test writing a medium sized file one byte at a time.
func TestMultiWrite(t *testing.T) {

	_, mnt := setupIpnsTest(t, nil)

	fpath := mnt.Dir + "/local/file"
	fi, err := os.Create(fpath)
	if err != nil {
		t.Fatal(err)
	}

	data := randBytes(1001)
	for i := range data {
		n, err := fi.Write(data[i : i+1])
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatal("Somehow wrote the wrong number of bytes! (n != 1)")
		}
	}
	fi.Close()

	rbuf, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(rbuf, data) {
		t.Fatal("File on disk did not match bytes written")
	}
}

// Test StoreMtime behavior: when enabled, written files get a recent mtime
// that persists across remounts. When disabled, mtime stays at zero/epoch.
func TestStoreMtime(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		_, mnt := setupIpnsTest(t, nil)

		fname := mnt.Dir + "/local/notime"
		writeFileOrFail(t, 100, fname)
		// Without StoreMtime, the underlying UnixFS node has no mtime.
		// Getattr returns zero, which the kernel shows as epoch.
		fi, err := os.Stat(fname)
		if err != nil {
			t.Fatal(err)
		}
		if fi.ModTime().After(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Fatalf("expected epoch-ish mtime without StoreMtime, got %v", fi.ModTime())
		}
	})

	t.Run("enabled", func(t *testing.T) {
		before := time.Now().Add(-time.Second)
		node, mnt := setupIpnsTest(t, nil, config.Mounts{StoreMtime: config.True})

		fname := mnt.Dir + "/local/withtime"
		writeFileOrFail(t, 100, fname)

		mnt.Close() // flush MFS roots before remounting

		// Remount and verify the mtime survived.
		_, mnt = setupIpnsTest(t, node)

		fi, err := os.Stat(mnt.Dir + "/local/withtime")
		if err != nil {
			t.Fatal(err)
		}
		if !fi.ModTime().After(before) {
			t.Fatalf("mtime did not persist across remount: got %v", fi.ModTime())
		}
	})
}

// Test that the default file mode matches the shared constant and chmod
// is ignored without StoreMode, but persists when StoreMode is enabled.
func TestStoreMode(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		_, mnt := setupIpnsTest(t, nil)

		fname := mnt.Dir + "/local/nomode"
		writeFileOrFail(t, 100, fname)
		fi, err := os.Stat(fname)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != fusemnt.DefaultFileModeRW.Perm() {
			t.Fatalf("expected default mode %04o, got %04o", fusemnt.DefaultFileModeRW.Perm(), fi.Mode().Perm())
		}
		// chmod should be silently ignored.
		_ = os.Chmod(fname, 0o755)
		fi, err = os.Stat(fname)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != fusemnt.DefaultFileModeRW.Perm() {
			t.Fatalf("mode changed without StoreMode: got %04o", fi.Mode().Perm())
		}
	})

	t.Run("enabled", func(t *testing.T) {
		_, mnt := setupIpnsTest(t, nil, config.Mounts{StoreMode: config.True})

		fname := mnt.Dir + "/local/withmode"
		writeFileOrFail(t, 100, fname)
		fi, err := os.Stat(fname)
		if err != nil {
			t.Fatal(err)
		}
		// Sanity: starting mode should differ from our target.
		if fi.Mode().Perm() == 0o755 {
			t.Fatal("new file already has 0755, cannot distinguish chmod effect")
		}
		if err := os.Chmod(fname, 0o755); err != nil {
			t.Fatal(err)
		}
		fi, err = os.Stat(fname)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o755 {
			t.Fatalf("expected mode 0755 after chmod, got %04o", fi.Mode().Perm())
		}
	})
}

// Test that directory mtime can be set (tar and rsync do this).
func TestDirMtime(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		_, mnt := setupIpnsTest(t, nil)

		dir := mnt.Dir + "/local/mtimedir"
		mkdir(t, dir)

		// utimensat must not error, but the value should not persist:
		// without StoreMtime, fillAttr never calls SetTimes and the
		// kernel reports epoch (Unix time 0).
		target := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(dir, target, target))

		fi, err := os.Stat(dir)
		require.NoError(t, err)
		require.Equal(t, time.Unix(0, 0), fi.ModTime(),
			"without StoreMtime, directory mtime should remain at epoch")
	})

	t.Run("enabled", func(t *testing.T) {
		_, mnt := setupIpnsTest(t, nil, config.Mounts{StoreMtime: config.True})

		dir := mnt.Dir + "/local/mtimedir"
		mkdir(t, dir)

		target := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(dir, target, target))

		fi, err := os.Stat(dir)
		require.NoError(t, err)
		require.Equal(t, target.Unix(), fi.ModTime().Unix(),
			"directory mtime should match what was set via utimensat")
	})
}

// Test that directory chmod works when StoreMode is enabled.
func TestDirChmod(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		_, mnt := setupIpnsTest(t, nil)

		dir := mnt.Dir + "/local/modedir"
		mkdir(t, dir)

		// chmod must not error, but mode should stay at the default.
		require.NoError(t, os.Chmod(dir, 0o700))
		fi, err := os.Stat(dir)
		require.NoError(t, err)
		require.Equal(t, fusemnt.DefaultDirModeRW.Perm(), fi.Mode().Perm(),
			"without StoreMode, directory mode should remain at fusemnt.DefaultDirModeRW")
	})

	t.Run("enabled", func(t *testing.T) {
		_, mnt := setupIpnsTest(t, nil, config.Mounts{StoreMode: config.True})

		dir := mnt.Dir + "/local/modedir"
		mkdir(t, dir)

		require.NoError(t, os.Chmod(dir, 0o700))
		fi, err := os.Stat(dir)
		require.NoError(t, err)
		require.Equal(t, iofs.FileMode(0o700), fi.Mode().Perm(),
			"directory mode should match what was set via chmod")
	})
}

// Test that directories get the expected default mode.
func TestDefaultDirMode(t *testing.T) {
	_, mnt := setupIpnsTest(t, nil)

	dir := mnt.Dir + "/local/subdir"
	mkdir(t, dir)

	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != fusemnt.DefaultDirModeRW.Perm() {
		t.Fatalf("expected dir mode %04o, got %04o", fusemnt.DefaultDirModeRW.Perm(), fi.Mode().Perm())
	}
}

// Test that the /ipns/ root has the namespace root mode (execute-only).
func TestNamespaceRootMode(t *testing.T) {
	_, mnt := setupIpnsTest(t, nil)

	fi, err := os.Stat(mnt.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode() != fusemnt.NamespaceRootMode {
		t.Fatalf("expected root mode %v, got %v", fusemnt.NamespaceRootMode, fi.Mode())
	}
}

// Test that ipfs.cid xattr is available on files and directories.
func TestXattrCID(t *testing.T) {
	nd, mnt := setupIpnsTest(t, nil)

	writeFileOrFail(t, 100, mnt.Dir+"/local/xattrfile")

	peerDir, ok := mnt.Root.LocalDirs[nd.Identity.String()]
	if !ok {
		t.Fatal("peer directory not found")
	}

	t.Run("directory", func(t *testing.T) {
		dest := make([]byte, 256)
		sz, errno := peerDir.Listxattr(t.Context(), dest)
		if errno != 0 {
			t.Fatalf("Listxattr: %v", errno)
		}
		if !bytes.Contains(dest[:sz], []byte(fusemnt.XattrCID)) {
			t.Fatal("ipfs.cid not listed")
		}

		sz, errno = peerDir.Getxattr(t.Context(), fusemnt.XattrCID, dest)
		if errno != 0 {
			t.Fatalf("Getxattr: %v", errno)
		}
		if sz == 0 {
			t.Fatal("empty CID")
		}
	})

	t.Run("file", func(t *testing.T) {
		child, err := peerDir.dir.Child("xattrfile")
		if err != nil {
			t.Fatal(err)
		}
		fileNode := &FileNode{fi: child.(*mfs.File), root: mnt.Root}

		dest := make([]byte, 256)
		sz, errno := fileNode.Listxattr(t.Context(), dest)
		if errno != 0 {
			t.Fatalf("Listxattr: %v", errno)
		}
		if !bytes.Contains(dest[:sz], []byte(fusemnt.XattrCID)) {
			t.Fatal("ipfs.cid not listed")
		}

		sz, errno = fileNode.Getxattr(t.Context(), fusemnt.XattrCID, dest)
		if errno != 0 {
			t.Fatalf("Getxattr: %v", errno)
		}
		if sz == 0 {
			t.Fatal("empty CID")
		}
	})
}

// Test fsync on an open file. Editors (vim, emacs) and databases call
// fsync after writing to ensure data reaches persistent storage before
// reporting success to the user.
func TestFsync(t *testing.T) {
	_, mnt := setupIpnsTest(t, nil)

	fpath := mnt.Dir + "/local/syncme"
	f, err := os.Create(fpath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("fsync test data")); err != nil {
		t.Fatal(err)
	}
	if err := f.Sync(); err != nil {
		t.Fatalf("fsync failed: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "fsync test data" {
		t.Fatalf("content after fsync: got %q", got)
	}
}

// Test ftruncate(fd, size) on an open file. `rsync --inplace` and
// database engines use ftruncate to shrink or grow files to exact
// sizes without rewriting them.
func TestFtruncate(t *testing.T) {
	_, mnt := setupIpnsTest(t, nil)

	fpath := mnt.Dir + "/local/truncme"
	writeFileOrFail(t, 1000, fpath)
	original, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.OpenFile(fpath, os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(500); err != nil {
		t.Fatalf("ftruncate failed: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 500 {
		t.Fatalf("expected 500 bytes after ftruncate, got %d", len(got))
	}
	if !bytes.Equal(got, original[:500]) {
		t.Fatal("content mismatch after ftruncate")
	}
}

// Test writing and reading a file larger than the default UnixFS chunk
// size (256 KiB), forcing a multi-block DAG. Streaming media playback
// and file copies depend on multi-block reads working correctly.
func TestLargeFile(t *testing.T) {
	_, mnt := setupIpnsTest(t, nil)

	fpath := mnt.Dir + "/local/largefile"
	data := randBytes(1024*1024 + 1) // 1 MiB + 1 byte

	writeFileOrFail(t, 0, fpath) // create empty file
	if err := os.WriteFile(fpath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("large file content mismatch: got %d bytes, want %d", len(got), len(data))
	}
}

// Test that getxattr on an unknown attribute returns an error.
// Tools like `cp -a` and `rsync -X` probe for xattrs and must handle
// ENODATA gracefully.
func TestUnknownXattr(t *testing.T) {
	nd, mnt := setupIpnsTest(t, nil)

	peerDir, ok := mnt.Root.LocalDirs[nd.Identity.String()]
	if !ok {
		t.Fatal("peer directory not found")
	}

	dest := make([]byte, 256)
	_, errno := peerDir.Getxattr(t.Context(), "user.bogus", dest)
	if errno == 0 {
		t.Fatal("expected error for unknown xattr, got success")
	}
}

// Test opening an existing file with O_TRUNC to replace its content.
// Editors like vim (with backupcopy=yes) use open(O_WRONLY|O_TRUNC)
// to overwrite the file in place.
func TestOpenTrunc(t *testing.T) {
	_, mnt := setupIpnsTest(t, nil)

	fpath := mnt.Dir + "/local/truncopen"
	writeFileOrFail(t, 100, fpath)

	f, err := os.OpenFile(fpath, os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("new")); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("expected %q, got %q", "new", got)
	}
}

// Test the atomic-save pattern used by rsync (default mode) and many
// editors: write to a temp file, then rename over the original.
// This ensures the target is never left in a half-written state.
func TestTempFileRename(t *testing.T) {
	_, mnt := setupIpnsTest(t, nil)

	target := mnt.Dir + "/local/target.txt"
	writeFileOrFail(t, 50, target)

	tmp := mnt.Dir + "/local/.target.tmp"
	newData := writeFileOrFail(t, 80, tmp)

	if err := os.Rename(tmp, target); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, newData) {
		t.Fatalf("content mismatch after rename: got %d bytes, want %d", len(got), len(newData))
	}

	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Fatalf("temp file still exists after rename: %v", err)
	}
}

// Test writing at an offset in the middle of a file. `rsync --inplace`
// uses pwrite to update changed blocks without rewriting the whole file.
func TestSeekAndWrite(t *testing.T) {
	_, mnt := setupIpnsTest(t, nil)

	fpath := mnt.Dir + "/local/seekwrite"
	writeFileOrFail(t, 0, fpath) // create the file

	// Write 10 'a' bytes.
	if err := os.WriteFile(fpath, []byte("aaaaaaaaaa"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := os.OpenFile(fpath, os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte("XXXX"), 3); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "aaaXXXXaaa" {
		t.Fatalf("expected %q, got %q", "aaaXXXXaaa", got)
	}
}

// Test reopening an existing file and writing different content.
// Log rotation, package managers, and CI artifacts overwrite files
// by opening them O_WRONLY without O_TRUNC (relying on the new
// content being the same size or followed by ftruncate).
func TestOverwriteExisting(t *testing.T) {
	_, mnt := setupIpnsTest(t, nil)

	fpath := mnt.Dir + "/local/overwrite"
	writeFileOrFail(t, 13, fpath)

	f, err := os.OpenFile(fpath, os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	replacement := []byte("second ver.!!")
	if _, err := f.Write(replacement); err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(int64(len(replacement))); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(replacement) {
		t.Fatalf("expected %q, got %q", replacement, got)
	}
}

// Test the exact save sequence vim uses: open with O_TRUNC, write
// new content, fsync, then chmod to restore permissions.
func TestVimSavePattern(t *testing.T) {
	_, mnt := setupIpnsTest(t, nil, config.Mounts{StoreMode: config.True})

	fpath := mnt.Dir + "/local/vimsave"
	writeFileOrFail(t, 50, fpath)

	f, err := os.OpenFile(fpath, os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("final version")); err != nil {
		t.Fatal(err)
	}
	if err := f.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(fpath, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "final version" {
		t.Fatalf("expected %q, got %q", "final version", got)
	}
	fi, err := os.Stat(fpath)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o644 {
		t.Fatalf("expected mode 0644, got %04o", fi.Mode().Perm())
	}
}

// Test the exact save sequence rsync uses (default mode): create a
// temp file with a dot prefix, write content, then rename over the
// original. This is how rsync achieves atomic updates.
func TestRsyncPattern(t *testing.T) {
	_, mnt := setupIpnsTest(t, nil)

	target := mnt.Dir + "/local/document.txt"
	writeFileOrFail(t, 50, target)

	tmp := mnt.Dir + "/local/.document.txt.XXXXXX"
	newData := writeFileOrFail(t, 80, tmp)

	if err := os.Rename(tmp, target); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, newData) {
		t.Fatalf("content mismatch: got %d bytes, want %d", len(got), len(newData))
	}
}

// Test creating and reading a relative symlink. Package managers,
// build systems, and tools like `ln -s` create symlinks inside the
// filesystem. The target is stored as a UnixFS TSymlink node.
func TestSymlink(t *testing.T) {
	_, mnt := setupIpnsTest(t, nil)

	// Create a file and a symlink pointing to it.
	target := mnt.Dir + "/local/real.txt"
	writeFileOrFail(t, 10, target)

	link := mnt.Dir + "/local/link.txt"
	if err := os.Symlink("real.txt", link); err != nil {
		t.Fatal(err)
	}

	got, err := os.Readlink(link)
	if err != nil {
		t.Fatal(err)
	}
	if got != "real.txt" {
		t.Fatalf("readlink: expected %q, got %q", "real.txt", got)
	}

	// Lstat should show a symlink.
	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("Lstat should report a symlink")
	}
}
