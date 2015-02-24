package readonly

import (
	"bytes"
	"crypto/rand"
	//context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"io/ioutil"
	"os"
	"path"
	"testing"

	fstest "github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse/fs/fstestutil"

	core "github.com/jbenet/go-ipfs/core"
	importer "github.com/jbenet/go-ipfs/importer"
	chunk "github.com/jbenet/go-ipfs/importer/chunk"
	dag "github.com/jbenet/go-ipfs/merkledag"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
	u "github.com/jbenet/go-ipfs/util"
	ci "github.com/jbenet/go-ipfs/util/testutil/ci"
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

func randObj(t *testing.T, nd *core.IpfsNode, size int64) (*dag.Node, []byte) {
	buf := make([]byte, size)
	u.NewTimeSeededRand().Read(buf)
	read := bytes.NewReader(buf)
	obj, err := importer.BuildTrickleDagFromReader(read, nd.DAG, nil, chunk.DefaultSplitter)
	if err != nil {
		t.Fatal(err)
	}

	return obj, buf
}

func setupIpfsTest(t *testing.T, node *core.IpfsNode) (*core.IpfsNode, *fstest.Mount) {
	maybeSkipFuseTests(t)

	var err error
	if node == nil {
		node, err = core.NewMockNode()
		if err != nil {
			t.Fatal(err)
		}
	}

	fs := NewFileSystem(node)
	mnt, err := fstest.MountedT(t, fs)
	if err != nil {
		t.Fatal(err)
	}

	return node, mnt
}

// Test writing an object and reading it back through fuse
func TestIpfsBasicRead(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	nd, mnt := setupIpfsTest(t, nil)
	defer mnt.Close()

	fi, data := randObj(t, nd, 10000)
	k, err := fi.Key()
	if err != nil {
		t.Fatal(err)
	}

	fname := path.Join(mnt.Dir, k.String())
	rbuf, err := ioutil.ReadFile(fname)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(rbuf, data) {
		t.Fatal("Incorrect Read!")
	}
}

// Test writing a file and reading it back
func TestIpfsBasicDirRead(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	nd, mnt := setupIpfsTest(t, nil)
	defer mnt.Close()

	// Make a 'file'
	fi, data := randObj(t, nd, 10000)
	k, err := fi.Key()
	if err != nil {
		t.Fatal(err)
	}

	// Make a directory and put that file in it
	db := uio.NewDirectory(nd.DAG)
	err = db.AddChild("actual", k)
	if err != nil {
		t.Fatal(err)
	}

	d1nd := db.GetNode()
	d1ndk, err := nd.DAG.Add(d1nd)
	if err != nil {
		t.Fatal(err)
	}

	dirname := path.Join(mnt.Dir, d1ndk.String())
	fname := path.Join(dirname, "actual")
	rbuf, err := ioutil.ReadFile(fname)
	if err != nil {
		t.Fatal(err)
	}

	dirents, err := ioutil.ReadDir(dirname)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirents) != 1 {
		t.Fatal("Bad directory entry count")
	}
	if dirents[0].Name() != "actual" {
		t.Fatal("Bad directory entry")
	}

	if !bytes.Equal(rbuf, data) {
		t.Fatal("Incorrect Read!")
	}
}

// Test to make sure the filesystem reports file sizes correctly
func TestFileSizeReporting(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	nd, mnt := setupIpfsTest(t, nil)
	defer mnt.Close()

	fi, data := randObj(t, nd, 10000)
	k, err := fi.Key()
	if err != nil {
		t.Fatal(err)
	}

	fname := path.Join(mnt.Dir, k.String())

	finfo, err := os.Stat(fname)
	if err != nil {
		t.Fatal(err)
	}

	if finfo.Size() != int64(len(data)) {
		t.Fatal("Read incorrect size from stat!")
	}
}
