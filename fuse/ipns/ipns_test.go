package ipns

import (
	"bytes"
	"crypto/rand"
	"io/ioutil"
	"os"
	"testing"
	"time"

	fstest "github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse/fs/fstestutil"
	"github.com/jbenet/go-ipfs/core"
)

func randBytes(size int) []byte {
	b := make([]byte, size)
	rand.Read(b)
	return b
}

func writeFile(t *testing.T, size int, path string) []byte {
	data := randBytes(size)
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

func setupIpnsTest(t *testing.T, node *core.IpfsNode) (*core.IpfsNode, *fstest.Mount) {
	var err error
	if node == nil {
		node, err = core.NewMockNode()
		if err != nil {
			t.Fatal(err)
		}
	}

	fs, err := NewIpns(node, "")
	if err != nil {
		t.Fatal(err)
	}
	mnt, err := fstest.MountedT(t, fs)
	if err != nil {
		t.Fatal(err)
	}

	return node, mnt
}

func TestIpnsBasicIO(t *testing.T) {
	_, mnt := setupIpnsTest(t, nil)
	defer mnt.Close()

	fname := mnt.Dir + "/local/testfile"
	data := writeFile(t, 12345, fname)

	rbuf, err := ioutil.ReadFile(fname)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(rbuf, data) {
		t.Fatal("Incorrect Read!")
	}
}

func TestFilePersistence(t *testing.T) {
	node, mnt := setupIpnsTest(t, nil)

	fname := "/local/atestfile"
	data := writeFile(t, 127, mnt.Dir+fname)

	// Wait for publish: TODO: make publish happen faster in tests
	time.Sleep(time.Millisecond * 40)

	mnt.Close()

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

func TestFileSizeReporting(t *testing.T) {
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
