package importer

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	chunk "github.com/ipfs/go-ipfs/importer/chunk"
	dag "github.com/ipfs/go-ipfs/merkledag"
	mdtest "github.com/ipfs/go-ipfs/merkledag/test"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
	u "github.com/ipfs/go-ipfs/util"
)

func getBalancedDag(t testing.TB, size int64, blksize int) (*dag.Node, dag.DAGService) {
	ds := mdtest.Mock(t)
	r := io.LimitReader(u.NewTimeSeededRand(), size)
	nd, err := BuildDagFromReader(r, ds, &chunk.SizeSplitter{blksize}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return nd, ds
}

func getTrickleDag(t testing.TB, size int64, blksize int) (*dag.Node, dag.DAGService) {
	ds := mdtest.Mock(t)
	r := io.LimitReader(u.NewTimeSeededRand(), size)
	nd, err := BuildTrickleDagFromReader(r, ds, &chunk.SizeSplitter{blksize}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return nd, ds
}

func TestBalancedDag(t *testing.T) {
	ds := mdtest.Mock(t)
	buf := make([]byte, 10000)
	u.NewTimeSeededRand().Read(buf)
	r := bytes.NewReader(buf)

	nd, err := BuildDagFromReader(r, ds, chunk.DefaultSplitter, nil)
	if err != nil {
		t.Fatal(err)
	}

	dr, err := uio.NewDagReader(context.TODO(), nd, ds)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ioutil.ReadAll(dr)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out, buf) {
		t.Fatal("bad read")
	}
}

func BenchmarkBalancedReadSmallBlock(b *testing.B) {
	b.StopTimer()
	nbytes := int64(10000000)
	nd, ds := getBalancedDag(b, nbytes, 4096)

	b.SetBytes(nbytes)
	b.StartTimer()
	runReadBench(b, nd, ds)
}

func BenchmarkTrickleReadSmallBlock(b *testing.B) {
	b.StopTimer()
	nbytes := int64(10000000)
	nd, ds := getTrickleDag(b, nbytes, 4096)

	b.SetBytes(nbytes)
	b.StartTimer()
	runReadBench(b, nd, ds)
}

func BenchmarkBalancedReadFull(b *testing.B) {
	b.StopTimer()
	nbytes := int64(10000000)
	nd, ds := getBalancedDag(b, nbytes, chunk.DefaultBlockSize)

	b.SetBytes(nbytes)
	b.StartTimer()
	runReadBench(b, nd, ds)
}

func BenchmarkTrickleReadFull(b *testing.B) {
	b.StopTimer()
	nbytes := int64(10000000)
	nd, ds := getTrickleDag(b, nbytes, chunk.DefaultBlockSize)

	b.SetBytes(nbytes)
	b.StartTimer()
	runReadBench(b, nd, ds)
}

func runReadBench(b *testing.B, nd *dag.Node, ds dag.DAGService) {
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithCancel(context.TODO())
		read, err := uio.NewDagReader(ctx, nd, ds)
		if err != nil {
			b.Fatal(err)
		}

		_, err = read.WriteTo(ioutil.Discard)
		if err != nil && err != io.EOF {
			b.Fatal(err)
		}
		cancel()
	}
}
