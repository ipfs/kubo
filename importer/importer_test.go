package importer

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	chunk "github.com/jbenet/go-ipfs/importer/chunk"
	dag "github.com/jbenet/go-ipfs/merkledag"
	mdtest "github.com/jbenet/go-ipfs/merkledag/test"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
	u "github.com/jbenet/go-ipfs/util"
)

func getBalancedDag(t testing.TB, size int64) (*dag.Node, dag.DAGService) {
	ds := mdtest.Mock(t)
	r := io.LimitReader(u.NewTimeSeededRand(), size)
	nd, err := BuildDagFromReader(r, ds, nil, chunk.DefaultSplitter)
	if err != nil {
		t.Fatal(err)
	}
	return nd, ds
}

func getTrickleDag(t testing.TB, size int64) (*dag.Node, dag.DAGService) {
	ds := mdtest.Mock(t)
	r := io.LimitReader(u.NewTimeSeededRand(), size)
	nd, err := BuildTrickleDagFromReader(r, ds, nil, chunk.DefaultSplitter)
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

	nd, err := BuildDagFromReader(r, ds, nil, chunk.DefaultSplitter)
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

func BenchmarkBalancedRead(b *testing.B) {
	b.StopTimer()
	nd, ds := getBalancedDag(b, int64(b.N))

	read, err := uio.NewDagReader(context.TODO(), nd, ds)
	if err != nil {
		b.Fatal(err)
	}

	b.StartTimer()
	b.SetBytes(int64(b.N))
	n, err := io.Copy(ioutil.Discard, read)
	if err != nil {
		b.Fatal(err)
	}
	if n != int64(b.N) {
		b.Fatal("Failed to read correct amount")
	}
}

func BenchmarkTrickleRead(b *testing.B) {
	b.StopTimer()
	nd, ds := getTrickleDag(b, int64(b.N))

	read, err := uio.NewDagReader(context.TODO(), nd, ds)
	if err != nil {
		b.Fatal(err)
	}

	b.StartTimer()
	b.SetBytes(int64(b.N))
	n, err := io.Copy(new(bytes.Buffer), read)
	if err != nil {
		b.Fatal(err)
	}
	if n != int64(b.N) {
		b.Fatal("Failed to read correct amount")
	}
}
