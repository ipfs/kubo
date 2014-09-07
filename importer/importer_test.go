package importer

import (
	"bytes"
	"crypto/rand"
	"io"
	"io/ioutil"
	"testing"

	dag "github.com/jbenet/go-ipfs/merkledag"
)

func TestFileConsistency(t *testing.T) {
	buf := new(bytes.Buffer)
	io.CopyN(buf, rand.Reader, 512*32)
	should := buf.Bytes()
	nd, err := NewDagFromReaderWithSplitter(buf, SplitterBySize(512))
	if err != nil {
		t.Fatal(err)
	}
	r, err := dag.NewDagReader(nd, nil)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out, should) {
		t.Fatal("Output not the same as input.")
	}
}

//Test where calls to read are smaller than the chunk size
func TestFileConsistencyLargeBlocks(t *testing.T) {
	buf := new(bytes.Buffer)
	io.CopyN(buf, rand.Reader, 4096*32)
	should := buf.Bytes()
	nd, err := NewDagFromReaderWithSplitter(buf, SplitterBySize(4096))
	if err != nil {
		t.Fatal(err)
	}
	r, err := dag.NewDagReader(nd, nil)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out, should) {
		t.Fatal("Output not the same as input.")
	}
}
