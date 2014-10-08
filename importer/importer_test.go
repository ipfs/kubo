package importer

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/jbenet/go-ipfs/importer/chunk"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
)

// NOTE:
// These tests tests a combination of unixfs/io/dagreader and importer/chunk.
// Maybe split them up somehow?
func TestBuildDag(t *testing.T) {
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
	bs := &chunk.SizeSplitter{512}
	testFileConsistency(t, bs, 32*512)
	bs = &chunk.SizeSplitter{4096}
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
	testFileConsistency(t, chunk.NewMaybeRabin(4096), 256*4096)
}

func TestRabinBlockSize(t *testing.T) {
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
