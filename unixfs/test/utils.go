package testu

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	h "github.com/ipfs/go-ipfs/importer/helpers"
	trickle "github.com/ipfs/go-ipfs/importer/trickle"
	mdag "github.com/ipfs/go-ipfs/merkledag"
	mdagmock "github.com/ipfs/go-ipfs/merkledag/test"
	ft "github.com/ipfs/go-ipfs/unixfs"

	u "gx/ipfs/QmNiJuT8Ja3hMVpBHXv3Q6dwmperaQ6JjLtpMQgMCD7xvx/go-ipfs-util"
	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	chunker "gx/ipfs/QmbGDSVKnYJZrtUnyxwsUpCeuigshNuVFxXCpv13jXecq1/go-ipfs-chunker"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

// SizeSplitterGen creates a generator.
func SizeSplitterGen(size int64) chunker.SplitterGen {
	return func(r io.Reader) chunker.Splitter {
		return chunker.NewSizeSplitter(r, size)
	}
}

// GetDAGServ returns a mock DAGService.
func GetDAGServ() ipld.DAGService {
	return mdagmock.Mock()
}

// NodeOpts is used by GetNode, GetEmptyNode and GetRandomNode
type NodeOpts struct {
	Prefix cid.Prefix
	// ForceRawLeaves if true will force the use of raw leaves
	ForceRawLeaves bool
	// RawLeavesUsed is true if raw leaves or either implicitly or explicitly enabled
	RawLeavesUsed bool
}

// Some shorthands for NodeOpts.
var (
	UseProtoBufLeaves = NodeOpts{Prefix: mdag.V0CidPrefix()}
	UseRawLeaves      = NodeOpts{Prefix: mdag.V0CidPrefix(), ForceRawLeaves: true, RawLeavesUsed: true}
	UseCidV1          = NodeOpts{Prefix: mdag.V1CidPrefix(), RawLeavesUsed: true}
	UseBlake2b256     NodeOpts
)

func init() {
	UseBlake2b256 = UseCidV1
	UseBlake2b256.Prefix.MhType = mh.Names["blake2b-256"]
	UseBlake2b256.Prefix.MhLength = -1
}

// GetNode returns a unixfs file node with the specified data.
func GetNode(t testing.TB, dserv ipld.DAGService, data []byte, opts NodeOpts) ipld.Node {
	in := bytes.NewReader(data)

	dbp := h.DagBuilderParams{
		Dagserv:   dserv,
		Maxlinks:  h.DefaultLinksPerBlock,
		Prefix:    &opts.Prefix,
		RawLeaves: opts.RawLeavesUsed,
	}

	node, err := trickle.Layout(dbp.New(SizeSplitterGen(500)(in)))
	if err != nil {
		t.Fatal(err)
	}

	return node
}

// GetEmptyNode returns an empty unixfs file node.
func GetEmptyNode(t testing.TB, dserv ipld.DAGService, opts NodeOpts) ipld.Node {
	return GetNode(t, dserv, []byte{}, opts)
}

// GetRandomNode returns a random unixfs file node.
func GetRandomNode(t testing.TB, dserv ipld.DAGService, size int64, opts NodeOpts) ([]byte, ipld.Node) {
	in := io.LimitReader(u.NewTimeSeededRand(), size)
	buf, err := ioutil.ReadAll(in)
	if err != nil {
		t.Fatal(err)
	}

	node := GetNode(t, dserv, buf, opts)
	return buf, node
}

// ArrComp checks if two byte slices are the same.
func ArrComp(a, b []byte) error {
	if len(a) != len(b) {
		return fmt.Errorf("arrays differ in length. %d != %d", len(a), len(b))
	}
	for i, v := range a {
		if v != b[i] {
			return fmt.Errorf("arrays differ at index: %d", i)
		}
	}
	return nil
}

// PrintDag pretty-prints the given dag to stdout.
func PrintDag(nd *mdag.ProtoNode, ds ipld.DAGService, indent int) {
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
