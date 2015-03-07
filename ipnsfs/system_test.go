package ipnsfs_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"testing"

	core "github.com/jbenet/go-ipfs/core"
	. "github.com/jbenet/go-ipfs/ipnsfs"
	u "github.com/jbenet/go-ipfs/util"
)

func testFS(t *testing.T, nd *core.IpfsNode) *Filesystem {
	fs, err := NewFilesystem(nd.Context(), nd.DAG, nd.Namesys, nd.Pinning, nd.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	return fs
}

func TestBasic(t *testing.T) {
	mock, err := core.NewMockNode()
	if err != nil {
		t.Fatal(err)
	}

	fs := testFS(t, mock)

	k := u.Key(mock.Identity)
	p := path.Join(k.B58String(), "file")
	fi, err := fs.Open(p, os.O_CREATE)
	if err != nil {
		t.Fatal(err)
	}

	data := []byte("Hello World")
	n, err := fi.Write(data)
	if err != nil {
		t.Fatal(err)
	}

	if n != len(data) {
		t.Fatal("wrote incorrect amount")
	}

	err = fi.Close()
	if err != nil {
		t.Fatal(err)
	}

	nfi, err := fs.Open(p, os.O_RDONLY)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ioutil.ReadAll(nfi)
	if err != nil {
		t.Fatal(err)
	}

	err = nfi.Close()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out, data) {
		t.Fatal("Write failed.")
	}

	err = fs.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Open the filesystem again, and try to read our file
	nfs := testFS(t, mock)

	fi, err = nfs.Open(p, os.O_RDONLY)
	nb, err := ioutil.ReadAll(fi)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(nb)

	if !bytes.Equal(nb, data) {
		t.Fatal("data not the same after closing down fs")
	}
}
