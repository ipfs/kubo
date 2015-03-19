package pin

import (
	"testing"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dssync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	"github.com/jbenet/go-ipfs/blocks/blockstore"
	bs "github.com/jbenet/go-ipfs/blockservice"
	"github.com/jbenet/go-ipfs/exchange/offline"
	mdag "github.com/jbenet/go-ipfs/merkledag"
	"github.com/jbenet/go-ipfs/util"
)

func randNode() (*mdag.Node, util.Key) {
	nd := new(mdag.Node)
	nd.Data = make([]byte, 32)
	util.NewTimeSeededRand().Read(nd.Data)
	k, _ := nd.Key()
	return nd, k
}

func TestPinnerBasic(t *testing.T) {
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	bstore := blockstore.NewBlockstore(dstore)
	bserv, err := bs.New(bstore, offline.Exchange(bstore))
	if err != nil {
		t.Fatal(err)
	}

	dserv := mdag.NewDAGService(bserv)

	// TODO does pinner need to share datastore with blockservice?
	p := NewPinner(dstore, dserv)

	a, ak := randNode()
	_, err = dserv.Add(a)
	if err != nil {
		t.Fatal(err)
	}

	// Pin A{}
	err = p.Pin(a, false)
	if err != nil {
		t.Fatal(err)
	}

	if !p.IsPinned(ak) {
		t.Fatal("Failed to find key")
	}

	// create new node c, to be indirectly pinned through b
	c, ck := randNode()
	_, err = dserv.Add(c)
	if err != nil {
		t.Fatal(err)
	}

	// Create new node b, to be parent to a and c
	b, _ := randNode()
	err = b.AddNodeLink("child", a)
	if err != nil {
		t.Fatal(err)
	}

	err = b.AddNodeLink("otherchild", c)
	if err != nil {
		t.Fatal(err)
	}

	_, err = dserv.Add(b)
	if err != nil {
		t.Fatal(err)
	}

	// recursively pin B{A,C}
	err = p.Pin(b, true)
	if err != nil {
		t.Fatal(err)
	}

	if !p.IsPinned(ck) {
		t.Fatal("Child of recursively pinned node not found")
	}

	bk, _ := b.Key()
	if !p.IsPinned(bk) {
		t.Fatal("Recursively pinned node not found..")
	}

	d, _ := randNode()
	d.AddNodeLink("a", a)
	d.AddNodeLink("c", c)

	e, ek := randNode()
	d.AddNodeLink("e", e)

	// Must be in dagserv for unpin to work
	err = dserv.AddRecursive(d)
	if err != nil {
		t.Fatal(err)
	}

	// Add D{A,C,E}
	err = p.Pin(d, true)
	if err != nil {
		t.Fatal(err)
	}

	if !p.IsPinned(ek) {
		t.Fatal(err)
	}

	dk, _ := d.Key()
	if !p.IsPinned(dk) {
		t.Fatal("pinned node not found.")
	}

	// Test recursive unpin
	err = p.Unpin(dk, true)
	if err != nil {
		t.Fatal(err)
	}

	// c should still be pinned under b
	if !p.IsPinned(ck) {
		t.Fatal("Recursive / indirect unpin fail.")
	}

	err = p.Flush()
	if err != nil {
		t.Fatal(err)
	}

	np, err := LoadPinner(dstore, dserv)
	if err != nil {
		t.Fatal(err)
	}

	// Test directly pinned
	if !np.IsPinned(ak) {
		t.Fatal("Could not find pinned node!")
	}

	// Test indirectly pinned
	if !np.IsPinned(ck) {
		t.Fatal("could not find indirectly pinned node")
	}

	// Test recursively pinned
	if !np.IsPinned(bk) {
		t.Fatal("could not find recursively pinned node")
	}
}

func TestDuplicateSemantics(t *testing.T) {
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	bstore := blockstore.NewBlockstore(dstore)
	bserv, err := bs.New(bstore, offline.Exchange(bstore))
	if err != nil {
		t.Fatal(err)
	}

	dserv := mdag.NewDAGService(bserv)

	// TODO does pinner need to share datastore with blockservice?
	p := NewPinner(dstore, dserv)

	a, _ := randNode()
	_, err = dserv.Add(a)
	if err != nil {
		t.Fatal(err)
	}

	// pin is recursively
	err = p.Pin(a, true)
	if err != nil {
		t.Fatal(err)
	}

	// pinning directly should fail
	err = p.Pin(a, false)
	if err == nil {
		t.Fatal("expected direct pin to fail")
	}

	// pinning recursively again should succeed
	err = p.Pin(a, true)
	if err != nil {
		t.Fatal(err)
	}
}
