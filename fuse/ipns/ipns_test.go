// +build !nofuse

package ipns

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	mrand "math/rand"
	"os"
	"sync"
	"testing"

	fstest "github.com/ipfs/go-ipfs/Godeps/_workspace/src/bazil.org/fuse/fs/fstestutil"
	racedet "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-detect-race"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	core "github.com/ipfs/go-ipfs/core"
	nsfs "github.com/ipfs/go-ipfs/ipnsfs"
	ci "github.com/ipfs/go-ipfs/util/testutil/ci"
)

func maybeSkipFuseTests(t *testing.T) {
	if ci.NoFuse() {
		t.Skip("Skipping FUSE tests")
	}
}

func randBytes(size int) []byte {
	b := make([]byte, size)
	rand.Read(b)
	return b
}

func mkdir(t *testing.T, path string) {
	err := os.Mkdir(path, os.ModeDir)
	if err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, size int, path string) []byte {
	return writeFileData(t, randBytes(size), path)
}

func writeFileData(t *testing.T, data []byte, path string) []byte {
	fi, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	n, err := fi.Write(data)
	if err != nil {
		t.Fatal(err)
	}

	if n != len(data) {
		t.Fatal("Didnt write proper amount!")
	}

	err = fi.Close()
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func verifyFile(t *testing.T, path string, data []byte) {
	fi, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fi.Close()

	buf := make([]byte, 1024)
	offset := 0
	for {
		n, err := fi.Read(buf)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(buf[:n], data[offset:offset+n]) {
			t.Fatal("Data not equal")
		}

		if n < len(buf) {
			break
		}

		offset += n
	}
}

func checkExists(t *testing.T, path string) {
	_, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
}

func closeMount(mnt *fstest.Mount) {
	if err := recover(); err != nil {
		log.Error("Recovered panic")
		log.Error(err)
	}
	mnt.Close()
}

func setupIpnsTest(t *testing.T, node *core.IpfsNode) (*core.IpfsNode, *fstest.Mount) {
	maybeSkipFuseTests(t)

	var err error
	if node == nil {
		node, err = core.NewMockNode()
		if err != nil {
			t.Fatal(err)
		}

		ipnsfs, err := nsfs.NewFilesystem(context.TODO(), node.DAG, node.Namesys, node.Pinning, node.PrivateKey)
		if err != nil {
			t.Fatal(err)
		}

		node.IpnsFs = ipnsfs
	}

	fs, err := NewFileSystem(node, node.PrivateKey, "", "")
	if err != nil {
		t.Fatal(err)
	}
	mnt, err := fstest.MountedT(t, fs)
	if err != nil {
		t.Fatal(err)
	}

	return node, mnt
}

func TestIpnsLocalLink(t *testing.T) {
	nd, mnt := setupIpnsTest(t, nil)
	defer mnt.Close()
	name := mnt.Dir + "/local"

	_, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}

	linksto, err := os.Readlink(name)
	if err != nil {
		t.Fatal(err)
	}

	if linksto != nd.Identity.Pretty() {
		t.Fatal("Link invalid")
	}
}

// Test writing a file and reading it back
func TestIpnsBasicIO(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	_, mnt := setupIpnsTest(t, nil)
	defer closeMount(mnt)

	fname := mnt.Dir + "/local/testfile"
	data := writeFile(t, 10, fname)

	rbuf, err := ioutil.ReadFile(fname)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(rbuf, data) {
		t.Fatal("Incorrect Read!")
	}
}

// Test to make sure file changes persist over mounts of ipns
func TestFilePersistence(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	node, mnt := setupIpnsTest(t, nil)

	fname := "/local/atestfile"
	data := writeFile(t, 127, mnt.Dir+fname)

	mnt.Close()

	t.Log("Closed, opening new fs")
	node, mnt = setupIpnsTest(t, node)
	defer mnt.Close()

	rbuf, err := ioutil.ReadFile(mnt.Dir + fname)
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
	data1 := writeFile(t, 4000, mnt.Dir+dir1+"/file1")

	verifyFile(t, mnt.Dir+dir1+"/file1", data1)

	t.Log("sub directory")
	mkdir(t, mnt.Dir+dir1+"/dir2")

	checkExists(t, mnt.Dir+dir1+"/dir2")

	t.Log("file in that subdirectory")
	data2 := writeFile(t, 5000, mnt.Dir+dir1+"/dir2/file2")

	verifyFile(t, mnt.Dir+dir1+"/dir2/file2", data2)

	mnt.Close()
	t.Log("closing mount, then restarting")

	_, mnt = setupIpnsTest(t, node)

	checkExists(t, mnt.Dir+dir1)

	verifyFile(t, mnt.Dir+dir1+"/file1", data1)

	verifyFile(t, mnt.Dir+dir1+"/dir2/file2", data2)
	mnt.Close()
}

// Test to make sure the filesystem reports file sizes correctly
func TestFileSizeReporting(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	_, mnt := setupIpnsTest(t, nil)
	defer mnt.Close()

	fname := mnt.Dir + "/local/sizecheck"
	data := writeFile(t, 5555, fname)

	finfo, err := os.Stat(fname)
	if err != nil {
		t.Fatal(err)
	}

	if finfo.Size() != int64(len(data)) {
		t.Fatal("Read incorrect size from stat!")
	}
}

// Test to make sure you cant create multiple entries with the same name
func TestDoubleEntryFailure(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	_, mnt := setupIpnsTest(t, nil)
	defer mnt.Close()

	dname := mnt.Dir + "/local/thisisadir"
	err := os.Mkdir(dname, 0777)
	if err != nil {
		t.Fatal(err)
	}

	err = os.Mkdir(dname, 0777)
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
	data := writeFile(t, 1300, fname)

	fi, err := os.OpenFile(fname, os.O_RDWR|os.O_APPEND, 0666)
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

	rbuf, err := ioutil.ReadFile(fname)
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
				out := writeFile(t, fileSize, mnt.Dir+fmt.Sprintf("/local/%dFILE%d", n, j))
				data[n][j] = out
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < nactors; i++ {
		for j := 0; j < filesPerActor; j++ {
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
	for i := 0; i < ndirWorkers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < ndirs; j++ {
				dirlock.RLock()
				n := mrand.Intn(len(dirs))
				dir := dirs[n]
				dirlock.RUnlock()

				newDir := fmt.Sprintf("%s/dir%d-%d", dir, worker, j)
				err := os.Mkdir(newDir, os.ModeDir)
				if err != nil {
					t.Fatal(err)
				}
				dirlock.Lock()
				dirs = append(dirs, newDir)
				dirlock.Unlock()
			}
		}(i)
	}

	// Spawn off workers to make files
	for i := 0; i < nfileWorkers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < nfiles; j++ {
				dirlock.RLock()
				n := mrand.Intn(len(dirs))
				dir := dirs[n]
				dirlock.RUnlock()

				newFileName := fmt.Sprintf("%s/file%d-%d", dir, worker, j)

				data := writeFile(t, 2000+mrand.Intn(5000), newFileName)
				filelock.Lock()
				files[newFileName] = data
				filelock.Unlock()
			}
		}(i)
	}

	wg.Wait()
	for name, data := range files {
		out, err := ioutil.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(data, out) {
			t.Fatal("Data didnt match")
		}
	}
}

/*
func TestFastRepublish(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	// make timeout noticeable.
	osrt := shortRepublishTimeout
	shortRepublishTimeout = time.Millisecond * 100

	olrt := longRepublishTimeout
	longRepublishTimeout = time.Second

	node, mnt := setupIpnsTest(t, nil)

	h, err := node.PrivateKey.GetPublic().Hash()
	if err != nil {
		t.Fatal(err)
	}
	pubkeyPath := "/ipns/" + u.Key(h).String()

	// set them back
	defer func() {
		shortRepublishTimeout = osrt
		longRepublishTimeout = olrt
		mnt.Close()
	}()

	closed := make(chan struct{})
	dataA := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	dataB := []byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")

	fname := mnt.Dir + "/local/file"

	// get first resolved hash
	log.Debug("publishing first hash")
	writeFileData(t, dataA, fname) // random
	<-time.After(shortRepublishTimeout * 2)
	log.Debug("resolving first hash")
	resolvedHash, err := node.Namesys.Resolve(context.Background(), pubkeyPath)
	if err != nil {
		t.Fatal("resolve err:", pubkeyPath, err)
	}

	// constantly keep writing to the file
	go func(timeout time.Duration) {
		for {
			select {
			case <-closed:
				return

			case <-time.After(timeout * 8 / 10):
				writeFileData(t, dataB, fname)
			}
		}
	}(shortRepublishTimeout)

	hasPublished := func() bool {
		res, err := node.Namesys.Resolve(context.Background(), pubkeyPath)
		if err != nil {
			t.Fatalf("resolve err: %v", err)
		}
		return res != resolvedHash
	}

	// test things

	// at this point, should not have written dataA and not have written dataB
	rbuf, err := ioutil.ReadFile(fname)
	if err != nil || !bytes.Equal(rbuf, dataA) {
		t.Fatalf("Data inconsistent! %v %v", err, string(rbuf))
	}

	if hasPublished() {
		t.Fatal("published (wrote)")
	}

	<-time.After(shortRepublishTimeout * 11 / 10)

	// at this point, should have written written dataB, but not published it
	rbuf, err = ioutil.ReadFile(fname)
	if err != nil || !bytes.Equal(rbuf, dataB) {
		t.Fatalf("Data inconsistent! %v %v", err, string(rbuf))
	}

	if hasPublished() {
		t.Fatal("published (wrote)")
	}

	<-time.After(longRepublishTimeout * 11 / 10)

	// at this point, should have written written dataB, and published it
	rbuf, err = ioutil.ReadFile(fname)
	if err != nil || !bytes.Equal(rbuf, dataB) {
		t.Fatalf("Data inconsistent! %v %v", err, string(rbuf))
	}

	if !hasPublished() {
		t.Fatal("not published")
	}

	close(closed)
}
*/

// Test writing a medium sized file one byte at a time
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
	for i := 0; i < len(data); i++ {
		n, err := fi.Write(data[i : i+1])
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatal("Somehow wrote the wrong number of bytes! (n != 1)")
		}
	}
	fi.Close()

	rbuf, err := ioutil.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(rbuf, data) {
		t.Fatal("File on disk did not match bytes written")
	}
}
