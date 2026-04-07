//go:build (linux || darwin || freebsd) && !nofuse

// Unit tests for the mutable /mfs FUSE mount.
// These test the filesystem implementation directly without a daemon.
// End-to-end tests that exercise mount/unmount through a real daemon
// live in test/cli/fuse/.

package mfs

import (
	"bytes"
	"context"
	"crypto/rand"
	iofs "io/fs"
	"os"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/node"
	"github.com/ipfs/kubo/fuse/fusetest"
	fusemnt "github.com/ipfs/kubo/fuse/mount"
)

func testMount(t *testing.T, root fs.InodeEmbedder) string {
	t.Helper()
	return fusetest.TestMount(t, root, &fs.Options{
		EntryTimeout: &mutableCacheTime,
		AttrTimeout:  &mutableCacheTime,
		MountOptions: fuse.MountOptions{
			MaxReadAhead:      fusemnt.MaxReadAhead,
			ExtraCapabilities: fusemnt.WritableMountCapabilities,
		},
	})
}

// Create an Ipfs.Node, a filesystem and a mount point.
func setUp(t *testing.T, ipfs *core.IpfsNode, cfgs ...config.Mounts) (*Dir, string) {
	t.Helper()

	var cfg config.Mounts
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}

	if ipfs == nil {
		var err error
		ipfs, err = core.NewNode(context.Background(), &node.BuildCfg{})
		if err != nil {
			t.Fatal(err)
		}
	}

	root := NewFileSystem(ipfs, cfg)
	mntDir := testMount(t, root)

	return root, mntDir
}

// Test reading and writing a file.
func TestReadWrite(t *testing.T) {
	_, mntDir := setUp(t, nil)

	path := mntDir + "/testrw"
	content := make([]byte, 8196)
	_, err := rand.Read(content)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("write", func(t *testing.T) {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		_, err = f.Write(content)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("read", func(t *testing.T) {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		buf := make([]byte, 8196)
		l, err := f.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(content, buf[:l]) {
			t.Fatal("read and write not equal")
		}
	})
}

// Test that empty directories can be listed without errors.
func TestEmptyDirListing(t *testing.T) {
	_, mntDir := setUp(t, nil)

	// The MFS root starts empty.
	entries, err := os.ReadDir(mntDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty root, got %d entries", len(entries))
	}

	// Create a directory and list it while still empty.
	dir := mntDir + "/emptydir"
	if err := os.Mkdir(dir, os.ModeDir); err != nil {
		t.Fatal(err)
	}
	entries, err = os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty directory, got %d entries", len(entries))
	}
}

// Test creating a directory.
func TestMkdir(t *testing.T) {
	_, mntDir := setUp(t, nil)

	path := mntDir + "/foo/bar/baz/qux/quux"

	t.Run("write", func(t *testing.T) {
		err := os.MkdirAll(path, iofs.ModeDir)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("read", func(t *testing.T) {
		stat, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !stat.IsDir() {
			t.Fatal("not dir")
		}
	})
}

// Test file persistence across mounts.
func TestPersistence(t *testing.T) {
	ipfs, err := core.NewNode(context.Background(), &node.BuildCfg{})
	if err != nil {
		t.Fatal(err)
	}

	content := make([]byte, 8196)
	_, err = rand.Read(content)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("write", func(t *testing.T) {
		_, mntDir := setUp(t, ipfs)
		path := mntDir + "/testpersistence"

		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		_, err = f.Write(content)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("read", func(t *testing.T) {
		_, mntDir := setUp(t, ipfs)
		path := mntDir + "/testpersistence"

		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		buf := make([]byte, 8196)
		l, err := f.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(content, buf[:l]) {
			t.Fatal("read and write not equal")
		}
	})
}

// Test getting the file attributes.
func TestAttr(t *testing.T) {
	_, mntDir := setUp(t, nil)

	path := mntDir + "/testattr"
	content := make([]byte, 8196)
	_, err := rand.Read(content)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("write", func(t *testing.T) {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		_, err = f.Write(content)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("read", func(t *testing.T) {
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}

		if fi.IsDir() {
			t.Fatal("file is a directory")
		}

		if fi.Name() != "testattr" {
			t.Fatal("invalid filename")
		}

		if fi.Size() != 8196 {
			t.Fatal("invalid size")
		}
	})
}

// Test concurrent access to the filesystem. Each file is written by
// one goroutine; after all writes complete, multiple goroutines read
// concurrently and verify the content.
func TestConcurrentRW(t *testing.T) {
	_, mntDir := setUp(t, nil)

	nfiles := 5
	readers := 5

	path := mntDir + "/testconcurrent"
	content := make([][]byte, nfiles)

	for i := range content {
		content[i] = make([]byte, 8196)
		_, err := rand.Read(content[i])
		if err != nil {
			t.Fatal(err)
		}
	}

	// Write phase: create all files and wait for Close (which flushes
	// data to the DAG) before moving on to reads.
	t.Run("write", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := range nfiles {
			wg.Go(func() {
				fname := path + strconv.Itoa(i)
				if err := os.WriteFile(fname, content[i], 0o644); err != nil {
					t.Error(err)
				}
			})
		}
		wg.Wait()
	})

	// Read phase: multiple goroutines read every file concurrently.
	t.Run("read", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < nfiles*readers; i++ {
			wg.Go(func() {
				fname := path + strconv.Itoa(i/readers)
				got, err := os.ReadFile(fname)
				if err != nil {
					t.Error(err)
					return
				}
				if !bytes.Equal(content[i/readers], got) {
					t.Error("read and write not equal")
				}
			})
		}
		wg.Wait()
	})
}

// Test appending data to an existing file.
func TestAppendFile(t *testing.T) {
	_, mntDir := setUp(t, nil)

	path := mntDir + "/appendfile"

	initial := make([]byte, 1300)
	if _, err := rand.Read(initial); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, initial, 0o644); err != nil {
		t.Fatal(err)
	}

	fi, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}

	extra := make([]byte, 500)
	if _, err := rand.Read(extra); err != nil {
		t.Fatal(err)
	}

	n, err := fi.Write(extra)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(extra) {
		t.Fatalf("short write: %d != %d", n, len(extra))
	}
	if err := fi.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := append(initial, extra...)
	if !bytes.Equal(got, want) {
		t.Fatalf("content mismatch: got %d bytes, want %d", len(got), len(want))
	}
}

// Test writing a file one byte at a time.
func TestMultiWrite(t *testing.T) {
	_, mntDir := setUp(t, nil)

	path := mntDir + "/multiwrite"
	fi, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	data := make([]byte, 1001)
	if _, err := rand.Read(data); err != nil {
		t.Fatal(err)
	}

	for i := range data {
		n, err := fi.Write(data[i : i+1])
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatal("short write")
		}
	}
	if err := fi.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("content mismatch")
	}
}

// Test renaming a file within the same directory.
func TestRenameFile(t *testing.T) {
	_, mntDir := setUp(t, nil)

	src := mntDir + "/before.txt"
	dst := mntDir + "/after.txt"

	data := make([]byte, 500)
	if _, err := rand.Read(data); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatal(err)
	}

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

// Test ipfs.cid extended attribute.
func TestMFSRootXattr(t *testing.T) {
	ipfs, err := core.NewNode(context.Background(), &node.BuildCfg{})
	if err != nil {
		t.Fatal(err)
	}

	root, _ := setUp(t, ipfs)

	dest := make([]byte, 256)
	sz, errno := root.Listxattr(context.Background(), dest)
	if errno != 0 {
		t.Fatalf("Listxattr: %v", errno)
	}
	if !bytes.Contains(dest[:sz], []byte(fusemnt.XattrCID)) {
		t.Fatalf("xattr list does not contain %s: %q", fusemnt.XattrCID, dest[:sz])
	}

	sz, errno = root.Getxattr(context.Background(), fusemnt.XattrCID, dest)
	if errno != 0 {
		t.Fatalf("Getxattr: %v", errno)
	}

	ipldNode, err := ipfs.FilesRoot.GetDirectory().GetNode()
	if err != nil {
		t.Fatal(err)
	}

	if slices.Compare(dest[:sz], []byte(ipldNode.Cid().String())) != 0 {
		t.Fatal("xattr cid not equal to mfs root cid")
	}
}

// Test StoreMtime behavior: when enabled, written files get a recent mtime
// that persists across remounts. When disabled, mtime stays at zero/epoch.
func TestStoreMtime(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		_, mntDir := setUp(t, nil)

		fname := mntDir + "/notime"
		if err := os.WriteFile(fname, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}
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
		ipfs, err := core.NewNode(context.Background(), &node.BuildCfg{})
		if err != nil {
			t.Fatal(err)
		}
		_, mntDir := setUp(t, ipfs, config.Mounts{StoreMtime: config.True})

		fname := mntDir + "/withtime"
		if err := os.WriteFile(fname, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Remount with default config and verify the mtime survived.
		_, mntDir2 := setUp(t, ipfs)

		fi, err := os.Stat(mntDir2 + "/withtime")
		if err != nil {
			t.Fatal(err)
		}
		if !fi.ModTime().After(before) {
			t.Fatalf("mtime did not persist across remount: got %v", fi.ModTime())
		}
	})
}

// Test that the default file mode matches the shared constant and chmod
// is ignored without StoreMode.
func TestStoreMode(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		_, mntDir := setUp(t, nil)

		fname := mntDir + "/nomode"
		if err := os.WriteFile(fname, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}
		fi, err := os.Stat(fname)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != fusemnt.DefaultFileModeRW.Perm() {
			t.Fatalf("expected default mode %04o, got %04o", fusemnt.DefaultFileModeRW.Perm(), fi.Mode().Perm())
		}
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
		_, mntDir := setUp(t, nil, config.Mounts{StoreMode: config.True})

		fname := mntDir + "/withmode"
		if err := os.WriteFile(fname, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}
		fi, err := os.Stat(fname)
		if err != nil {
			t.Fatal(err)
		}
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

// Test that directories get the expected default mode.
func TestDefaultDirMode(t *testing.T) {
	_, mntDir := setUp(t, nil)

	dir := mntDir + "/subdir"
	if err := os.Mkdir(dir, os.ModeDir); err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != fusemnt.DefaultDirModeRW.Perm() {
		t.Fatalf("expected dir mode %04o, got %04o", fusemnt.DefaultDirModeRW.Perm(), fi.Mode().Perm())
	}
}

// Test removing an empty directory (rmdir). Tools like `rm -r` and
// build systems remove directories bottom-up after deleting children.
func TestRmdir(t *testing.T) {
	_, mntDir := setUp(t, nil)

	dir := mntDir + "/mydir"
	if err := os.Mkdir(dir, os.ModeDir); err != nil {
		t.Fatal(err)
	}

	// Put a file inside, then try to rmdir (should fail).
	if err := os.WriteFile(dir+"/child", []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(dir); err == nil {
		t.Fatal("expected error removing non-empty directory")
	}

	// Remove the child, then rmdir should succeed.
	if err := os.Remove(dir + "/child"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("directory still exists after rmdir: %v", err)
	}
}

// Test fsync on an open file. Editors (vim, emacs) and databases call
// fsync after writing to ensure data reaches persistent storage before
// reporting success to the user.
func TestFsync(t *testing.T) {
	_, mntDir := setUp(t, nil)

	fpath := mntDir + "/syncme"
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
	_, mntDir := setUp(t, nil)

	fpath := mntDir + "/truncme"
	original := make([]byte, 1000)
	if _, err := rand.Read(original); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fpath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	// Open the file, truncate to 500 bytes via ftruncate.
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
	_, mntDir := setUp(t, nil)

	fpath := mntDir + "/largefile"
	size := 1024*1024 + 1 // 1 MiB + 1 byte, well above the 256 KiB chunk size
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		t.Fatal(err)
	}
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

// Test renaming a file from one directory to another. Package managers
// and `mv` across directories use cross-directory rename.
func TestCrossDirRename(t *testing.T) {
	_, mntDir := setUp(t, nil)

	srcDir := mntDir + "/srcdir"
	dstDir := mntDir + "/dstdir"
	if err := os.Mkdir(srcDir, os.ModeDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dstDir, os.ModeDir); err != nil {
		t.Fatal(err)
	}

	data := make([]byte, 500)
	if _, err := rand.Read(data); err != nil {
		t.Fatal(err)
	}
	src := srcDir + "/file.txt"
	dst := dstDir + "/moved.txt"
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Rename(src, dst); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source still exists after cross-dir rename: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("content mismatch: got %d bytes, want %d", len(got), len(data))
	}
}

// Test that getxattr on an unknown attribute returns an error.
// Tools like `cp -a` and `rsync -X` probe for xattrs and must handle
// ENODATA gracefully.
func TestUnknownXattr(t *testing.T) {
	root, _ := setUp(t, nil)

	dest := make([]byte, 256)
	_, errno := root.Getxattr(context.Background(), "user.bogus", dest)
	if errno == 0 {
		t.Fatal("expected error for unknown xattr, got success")
	}
}

// Test opening an existing file with O_TRUNC to replace its content.
// Editors like vim (with backupcopy=yes) use open(O_WRONLY|O_TRUNC)
// to overwrite the file in place.
func TestOpenTrunc(t *testing.T) {
	_, mntDir := setUp(t, nil)

	fpath := mntDir + "/truncopen"
	if err := os.WriteFile(fpath, []byte("original content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Reopen with O_TRUNC and write new content.
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

func TestTempFileRename(t *testing.T) {
	_, mntDir := setUp(t, nil)

	target := mntDir + "/target.txt"
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	tmp := mntDir + "/.target.tmp"
	if err := os.WriteFile(tmp, []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Rename(tmp, target); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new content" {
		t.Fatalf("expected %q after rename, got %q", "new content", got)
	}

	// After rename, the old name must not appear in directory listing
	// and stat must return ENOENT.
	entries, err := os.ReadDir(mntDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == ".target.tmp" {
			t.Fatal("source still appears in readdir after rename")
		}
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Fatalf("stat on old name should return ENOENT, got: %v", err)
	}
}

// Test writing at an offset in the middle of a file. `rsync --inplace`
// uses pwrite to update changed blocks without rewriting the whole file.
func TestSeekAndWrite(t *testing.T) {
	_, mntDir := setUp(t, nil)

	fpath := mntDir + "/seekwrite"
	original := []byte("aaaaaaaaaa") // 10 bytes
	if err := os.WriteFile(fpath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := os.OpenFile(fpath, os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	// Overwrite bytes 3..6 with "XXXX".
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
	want := []byte("aaaXXXXaaa")
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

// Test reopening an existing file and writing different content.
// Log rotation, package managers, and CI artifacts overwrite files
// by opening them O_WRONLY without O_TRUNC (relying on the new
// content being the same size or followed by ftruncate).
func TestOverwriteExisting(t *testing.T) {
	_, mntDir := setUp(t, nil)

	fpath := mntDir + "/overwrite"
	if err := os.WriteFile(fpath, []byte("first version"), 0o644); err != nil {
		t.Fatal(err)
	}

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
//
func TestVimSavePattern(t *testing.T) {
	_, mntDir := setUp(t, nil, config.Mounts{StoreMode: config.True})

	fpath := mntDir + "/vimsave"
	if err := os.WriteFile(fpath, []byte("draft"), 0o644); err != nil {
		t.Fatal(err)
	}

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
	_, mntDir := setUp(t, nil)

	target := mntDir + "/document.txt"
	if err := os.WriteFile(target, []byte("version 1"), 0o644); err != nil {
		t.Fatal(err)
	}

	tmp := mntDir + "/.document.txt.XXXXXX"
	if err := os.WriteFile(tmp, []byte("version 2"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Rename(tmp, target); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "version 2" {
		t.Fatalf("expected %q, got %q", "version 2", got)
	}
}


