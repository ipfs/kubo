package importer

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"

	chunk "github.com/jbenet/go-ipfs/importer/chunk"
	merkledag "github.com/jbenet/go-ipfs/merkledag"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
	u "github.com/jbenet/go-ipfs/util"
)

// NOTE:
// These tests tests a combination of unixfs/io/dagreader and importer/chunk.
// Maybe split them up somehow?
func TestBuildDag(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	td := os.TempDir()
	fi, err := os.Create(td + "/tmpfi")
	if err != nil {
		t.Fatal(err)
	}

	_, err = io.CopyN(fi, rand.Reader, 1024*1024)
	if err != nil {
		t.Fatal(err)
	}

	fi.Close()

	_, err = NewDagFromFile(td + "/tmpfi")
	if err != nil {
		t.Fatal(err)
	}
}

//Test where calls to read are smaller than the chunk size
func TestSizeBasedSplit(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	bs := &chunk.SizeSplitter{Size: 512}
	testFileConsistency(t, bs, 32*512)
	bs = &chunk.SizeSplitter{Size: 4096}
	testFileConsistency(t, bs, 32*4096)

	// Uneven offset
	testFileConsistency(t, bs, 31*4095)
}

func dup(b []byte) []byte {
	o := make([]byte, len(b))
	copy(o, b)
	return o
}

func testFileConsistency(t *testing.T, bs chunk.BlockSplitter, nbytes int) {
	buf := new(bytes.Buffer)
	io.CopyN(buf, rand.Reader, int64(nbytes))
	should := dup(buf.Bytes())
	nd, err := NewDagFromReaderWithSplitter(buf, bs)
	if err != nil {
		t.Fatal(err)
	}

	r, err := uio.NewDagReader(nd, nil)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	err = arrComp(out, should)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBuilderConsistency(t *testing.T) {
	nbytes := 100000
	buf := new(bytes.Buffer)
	io.CopyN(buf, u.NewTimeSeededRand(), int64(nbytes))
	should := dup(buf.Bytes())
	dagserv := merkledag.Mock(t)
	nd, err := BuildDagFromReader(buf, dagserv, nil, chunk.DefaultSplitter)
	if err != nil {
		t.Fatal(err)
	}
	r, err := uio.NewDagReader(nd, dagserv)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	err = arrComp(out, should)
	if err != nil {
		t.Fatal(err)
	}
}

func arrComp(a, b []byte) error {
	if len(a) != len(b) {
		return fmt.Errorf("Arrays differ in length. %d != %d", len(a), len(b))
	}
	for i, v := range a {
		if v != b[i] {
			return fmt.Errorf("Arrays differ at index: %d", i)
		}
	}
	return nil
}

func TestMaybeRabinConsistency(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	testFileConsistency(t, chunk.NewMaybeRabin(4096), 256*4096)
}

func TestRabinBlockSize(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	buf := new(bytes.Buffer)
	nbytes := 1024 * 1024
	io.CopyN(buf, rand.Reader, int64(nbytes))
	rab := chunk.NewMaybeRabin(4096)
	blkch := rab.Split(buf)

	var blocks [][]byte
	for b := range blkch {
		blocks = append(blocks, b)
	}

	fmt.Printf("Avg block size: %d\n", nbytes/len(blocks))

}
