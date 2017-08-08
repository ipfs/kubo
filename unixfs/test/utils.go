package testu

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	"github.com/ipfs/go-ipfs/importer/chunk"
	h "github.com/ipfs/go-ipfs/importer/helpers"
	trickle "github.com/ipfs/go-ipfs/importer/trickle"
	mdag "github.com/ipfs/go-ipfs/merkledag"
	mdagmock "github.com/ipfs/go-ipfs/merkledag/test"
	ft "github.com/ipfs/go-ipfs/unixfs"

	node "gx/ipfs/QmPN7cwmpcc4DWXb4KTB9dNAJgjuPY69h3npsMfhRrQL9c/go-ipld-format"
	u "gx/ipfs/QmSU6eubNdhXjFBJBSksTp8kv8YRub8mGAPv8tVJHmL2EU/go-ipfs-util"
)

func SizeSplitterGen(size int64) chunk.SplitterGen {
	return func(r io.Reader) chunk.Splitter {
		return chunk.NewSizeSplitter(r, size)
	}
}

func GetDAGServ() mdag.DAGService {
	return mdagmock.Mock()
}

type UseRawLeaves bool

const (
	ProtoBufLeaves UseRawLeaves = false
	RawLeaves      UseRawLeaves = true
)

func GetNode(t testing.TB, dserv mdag.DAGService, data []byte, rawLeaves UseRawLeaves) node.Node {
	in := bytes.NewReader(data)

	dbp := h.DagBuilderParams{
		Dagserv:   dserv,
		Maxlinks:  h.DefaultLinksPerBlock,
		RawLeaves: bool(rawLeaves),
	}

	node, err := trickle.TrickleLayout(dbp.New(SizeSplitterGen(500)(in)))
	if err != nil {
		t.Fatal(err)
	}

	return node
}

func GetEmptyNode(t testing.TB, dserv mdag.DAGService, rawLeaves UseRawLeaves) node.Node {
	return GetNode(t, dserv, []byte{}, rawLeaves)
}

func GetRandomNode(t testing.TB, dserv mdag.DAGService, size int64, rawLeaves UseRawLeaves) ([]byte, node.Node) {
	in := io.LimitReader(u.NewTimeSeededRand(), size)
	buf, err := ioutil.ReadAll(in)
	if err != nil {
		t.Fatal(err)
	}

	node := GetNode(t, dserv, buf, rawLeaves)
	return buf, node
}

func ArrComp(a, b []byte) error {
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

func PrintDag(nd *mdag.ProtoNode, ds mdag.DAGService, indent int) {
	pbd, err := ft.FromBytes(nd.Data())
	if err != nil {
		panic(err)
	}

	for i := 0; i < indent; i++ {
		fmt.Print(" ")
	}
	fmt.Printf("{size = %d, type = %s, children = %d", pbd.GetFilesize(), pbd.GetType().String(), len(pbd.GetBlocksizes()))
	if len(nd.Links()) > 0 {
		fmt.Println()
	}
	for _, lnk := range nd.Links() {
		child, err := lnk.GetNode(context.Background(), ds)
		if err != nil {
			panic(err)
		}
		PrintDag(child.(*mdag.ProtoNode), ds, indent+1)
	}
	if len(nd.Links()) > 0 {
		for i := 0; i < indent; i++ {
			fmt.Print(" ")
		}
	}
	fmt.Println("}")
}
