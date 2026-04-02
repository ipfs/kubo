//go:build !nofuse && !openbsd && !netbsd && !plan9

package ipns

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	mrand "math/rand"
	"os"
	"sync"
	"syscall"
	"testing"

	core "github.com/ipfs/kubo/core"
	coreapi "github.com/ipfs/kubo/core/coreapi"

	fstest "bazil.org/fuse/fs/fstestutil"
	racedet "github.com/ipfs/go-detect-race"
	"github.com/ipfs/go-test/random"
	"github.com/ipfs/kubo/fuse/fusetest"
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
	// Same flags as os.WriteFile: write-only, create if missing, truncate if exists.
	// We open manually instead of using os.WriteFile so we can handle EINTR on close.
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
	// daemon. Go intentionally does not retry close() on EINTR because the
	// fd is already closed on Linux and its state is undefined on other
	// platforms, making retry unsafe.
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

func closeMount(mnt *mountWrap) {
	if err := recover(); err != nil {
		log.Error("Recovered panic")
		log.Error(err)
	}
	mnt.Close()
}

type mountWrap struct {
	*fstest.Mount
	Fs *FileSystem
}

func (m *mountWrap) Close() error {
	m.Fs.Destroy()
	m.Mount.Close()
	return nil
}

func setupIpnsTest(t *testing.T, node *core.IpfsNode) (*core.IpfsNode, *mountWrap) {
	t.Helper()
	fusetest.SkipUnlessFUSE(t)

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

	fs, err := NewFileSystem(node.Context(), coreAPI, "", "")
	if err != nil {
		t.Fatal(err)
	}
	mnt, err := fstest.MountedT(t, fs, nil)
	fusetest.MountError(t, err)

	return node, &mountWrap{
		Mount: mnt,
		Fs:    fs,
	}
}

func TestIpnsLocalLink(t *testing.T) {
	nd, mnt := setupIpnsTest(t, nil)
	defer mnt.Close()
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
	defer mnt.Close()

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
	if testing.Short() {
		t.SkipNow()
	}
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
	if testing.Short() {
		t.SkipNow()
	}
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

// Test to make sure file changes persist over mounts of ipns.
func TestFilePersistence(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	node, mnt := setupIpnsTest(t, nil)

	fname := "/local/atestfile"
	data := writeFileOrFail(t, 127, mnt.Dir+fname)

	mnt.Close()

	t.Log("Closed, opening new fs")
	_, mnt = setupIpnsTest(t, node)
	defer mnt.Close()

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

	mnt.Close()
	t.Log("closing mount, then restarting")

	_, mnt = setupIpnsTest(t, node)

	checkExists(t, mnt.Dir+dir1)

	verifyFile(t, mnt.Dir+dir1+"/file1", data1)

	verifyFile(t, mnt.Dir+dir1+"/dir2/file2", data2)
	mnt.Close()
}

// Test to make sure the filesystem reports file sizes correctly.
func TestFileSizeReporting(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	_, mnt := setupIpnsTest(t, nil)
	defer mnt.Close()

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
	if testing.Short() {
		t.SkipNow()
	}
	_, mnt := setupIpnsTest(t, nil)
	defer mnt.Close()

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
	if testing.Short() {
		t.SkipNow()
	}
	_, mnt := setupIpnsTest(t, nil)
	defer mnt.Close()

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
	if testing.Short() {
		t.SkipNow()
	}
	_, mnt := setupIpnsTest(t, nil)
	defer mnt.Close()

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

	if testing.Short() {
		t.SkipNow()
	}
	_, mnt := setupIpnsTest(t, nil)
	defer mnt.Close()

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
	if testing.Short() {
		t.SkipNow()
	}

	_, mnt := setupIpnsTest(t, nil)
	defer mnt.Close()

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
