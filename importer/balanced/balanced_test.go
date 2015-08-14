package balanced

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	mrand "math/rand"
	"os"
	"testing"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	chunk "github.com/ipfs/go-ipfs/importer/chunk"
	h "github.com/ipfs/go-ipfs/importer/helpers"
	dag "github.com/ipfs/go-ipfs/merkledag"
	mdtest "github.com/ipfs/go-ipfs/merkledag/test"
	pin "github.com/ipfs/go-ipfs/pin"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
	u "github.com/ipfs/go-ipfs/util"
)

// TODO: extract these tests and more as a generic layout test suite

func buildTestDag(ds dag.DAGService, spl chunk.Splitter) (*dag.Node, error) {
	// Start the splitter
	blkch, errs := chunk.Chan(spl)

	dbp := h.DagBuilderParams{
		Dagserv:  ds,
		Maxlinks: h.DefaultLinksPerBlock,
	}

	return BalancedLayout(dbp.New(blkch, errs))
}

func getTestDag(t *testing.T, ds dag.DAGService, size int64, blksize int64) (*dag.Node, []byte) {
	data := make([]byte, size)
	u.NewTimeSeededRand().Read(data)
	r := bytes.NewReader(data)

	nd, err := buildTestDag(ds, chunk.NewSizeSplitter(r, blksize))
	if err != nil {
		t.Fatal(err)
	}

	return nd, data
}

//Test where calls to read are smaller than the chunk size
func TestSizeBasedSplit(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	bs := chunk.SizeSplitterGen(512)
	testFileConsistency(t, bs, 32*512)
	bs = chunk.SizeSplitterGen(4096)
	testFileConsistency(t, bs, 32*4096)

	// Uneven offset
	testFileConsistency(t, bs, 31*4095)
}

func dup(b []byte) []byte {
	o := make([]byte, len(b))
	copy(o, b)
	return o
}

func testFileConsistency(t *testing.T, bs chunk.SplitterGen, nbytes int) {
	should := make([]byte, nbytes)
	u.NewTimeSeededRand().Read(should)

	read := bytes.NewReader(should)
	ds := mdtest.Mock()
	nd, err := buildTestDag(ds, bs(read))
	if err != nil {
		t.Fatal(err)
	}

	r, err := uio.NewDagReader(context.Background(), nd, ds)
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
	dagserv := mdtest.Mock()
	nd, should := getTestDag(t, dagserv, 100000, chunk.DefaultBlockSize)

	r, err := uio.NewDagReader(context.Background(), nd, dagserv)
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

type dagservAndPinner struct {
	ds dag.DAGService
	mp pin.ManualPinner
}

func TestIndirectBlocks(t *testing.T) {
	ds := mdtest.Mock()
	dag, buf := getTestDag(t, ds, 1024*1024, 512)

	reader, err := uio.NewDagReader(context.Background(), dag, ds)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ioutil.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out, buf) {
		t.Fatal("Not equal!")
	}
}

func TestSeekingBasic(t *testing.T) {
	nbytes := int64(10 * 1024)
	ds := mdtest.Mock()
	nd, should := getTestDag(t, ds, nbytes, 500)

	rs, err := uio.NewDagReader(context.Background(), nd, ds)
	if err != nil {
		t.Fatal(err)
	}

	start := int64(4000)
	n, err := rs.Seek(start, os.SEEK_SET)
	if err != nil {
		t.Fatal(err)
	}
	if n != start {
		t.Fatal("Failed to seek to correct offset")
	}

	out, err := ioutil.ReadAll(rs)
	if err != nil {
		t.Fatal(err)
	}

	err = arrComp(out, should[start:])
	if err != nil {
		t.Fatal(err)
	}
}

func TestSeekToBegin(t *testing.T) {
	ds := mdtest.Mock()
	nd, should := getTestDag(t, ds, 10*1024, 500)

	rs, err := uio.NewDagReader(context.Background(), nd, ds)
	if err != nil {
		t.Fatal(err)
	}

	n, err := io.CopyN(ioutil.Discard, rs, 1024*4)
	if err != nil {
		t.Fatal(err)
	}
	if n != 4096 {
		t.Fatal("Copy didnt copy enough bytes")
	}

	seeked, err := rs.Seek(0, os.SEEK_SET)
	if err != nil {
		t.Fatal(err)
	}
	if seeked != 0 {
		t.Fatal("Failed to seek to beginning")
	}

	out, err := ioutil.ReadAll(rs)
	if err != nil {
		t.Fatal(err)
	}

	err = arrComp(out, should)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSeekToAlmostBegin(t *testing.T) {
	ds := mdtest.Mock()
	nd, should := getTestDag(t, ds, 10*1024, 500)

	rs, err := uio.NewDagReader(context.Background(), nd, ds)
	if err != nil {
		t.Fatal(err)
	}

	n, err := io.CopyN(ioutil.Discard, rs, 1024*4)
	if err != nil {
		t.Fatal(err)
	}
	if n != 4096 {
		t.Fatal("Copy didnt copy enough bytes")
	}

	seeked, err := rs.Seek(1, os.SEEK_SET)
	if err != nil {
		t.Fatal(err)
	}
	if seeked != 1 {
		t.Fatal("Failed to seek to almost beginning")
	}

	out, err := ioutil.ReadAll(rs)
	if err != nil {
		t.Fatal(err)
	}

	err = arrComp(out, should[1:])
	if err != nil {
		t.Fatal(err)
	}
}

func TestSeekEnd(t *testing.T) {
	nbytes := int64(50 * 1024)
	ds := mdtest.Mock()
	nd, _ := getTestDag(t, ds, nbytes, 500)

	rs, err := uio.NewDagReader(context.Background(), nd, ds)
	if err != nil {
		t.Fatal(err)
	}

	seeked, err := rs.Seek(0, os.SEEK_END)
	if err != nil {
		t.Fatal(err)
	}
	if seeked != nbytes {
		t.Fatal("Failed to seek to end")
	}
}

func TestSeekEndSingleBlockFile(t *testing.T) {
	nbytes := int64(100)
	ds := mdtest.Mock()
	nd, _ := getTestDag(t, ds, nbytes, 5000)

	rs, err := uio.NewDagReader(context.Background(), nd, ds)
	if err != nil {
		t.Fatal(err)
	}

	seeked, err := rs.Seek(0, os.SEEK_END)
	if err != nil {
		t.Fatal(err)
	}
	if seeked != nbytes {
		t.Fatal("Failed to seek to end")
	}
}

func TestSeekingStress(t *testing.T) {
	nbytes := int64(1024 * 1024)
	ds := mdtest.Mock()
	nd, should := getTestDag(t, ds, nbytes, 1000)

	rs, err := uio.NewDagReader(context.Background(), nd, ds)
	if err != nil {
		t.Fatal(err)
	}

	testbuf := make([]byte, nbytes)
	for i := 0; i < 50; i++ {
		offset := mrand.Intn(int(nbytes))
		l := int(nbytes) - offset
		n, err := rs.Seek(int64(offset), os.SEEK_SET)
		if err != nil {
			t.Fatal(err)
		}
		if n != int64(offset) {
			t.Fatal("Seek failed to move to correct position")
		}

		nread, err := rs.Read(testbuf[:l])
		if err != nil {
			t.Fatal(err)
		}
		if nread != l {
			t.Fatal("Failed to read enough bytes")
		}

		err = arrComp(testbuf[:l], should[offset:offset+l])
		if err != nil {
			t.Fatal(err)
		}
	}

}

func TestSeekingConsistency(t *testing.T) {
	nbytes := int64(128 * 1024)
	ds := mdtest.Mock()
	nd, should := getTestDag(t, ds, nbytes, 500)

	rs, err := uio.NewDagReader(context.Background(), nd, ds)
	if err != nil {
		t.Fatal(err)
	}

	out := make([]byte, nbytes)

	for coff := nbytes - 4096; coff >= 0; coff -= 4096 {
		t.Log(coff)
		n, err := rs.Seek(coff, os.SEEK_SET)
		if err != nil {
			t.Fatal(err)
		}
		if n != coff {
			t.Fatal("wasnt able to seek to the right position")
		}
		nread, err := rs.Read(out[coff : coff+4096])
		if err != nil {
			t.Fatal(err)
		}
		if nread != 4096 {
			t.Fatal("didnt read the correct number of bytes")
		}
	}

	err = arrComp(out, should)
	if err != nil {
		t.Fatal(err)
	}
}
