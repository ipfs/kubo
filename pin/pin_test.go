package pin

import (
	"testing"
	"time"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dssync "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	"github.com/ipfs/go-ipfs/blocks/blockstore"
	key "github.com/ipfs/go-ipfs/blocks/key"
	bs "github.com/ipfs/go-ipfs/blockservice"
	"github.com/ipfs/go-ipfs/exchange/offline"
	mdag "github.com/ipfs/go-ipfs/merkledag"
	"github.com/ipfs/go-ipfs/util"
)

func randNode() (*mdag.Node, key.Key) {
	nd := new(mdag.Node)
	nd.Data = make([]byte, 32)
	util.NewTimeSeededRand().Read(nd.Data)
	k, _ := nd.Key()
	return nd, k
}

func TestPinnerBasic(t *testing.T) {
	ctx := context.Background()

	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	bstore := blockstore.NewBlockstore(dstore)
	bserv := bs.New(bstore, offline.Exchange(bstore))

	dserv := mdag.NewDAGService(bserv)

	// TODO does pinner need to share datastore with blockservice?
	p := NewPinner(dstore, dserv)

	a, ak := randNode()
	_, err := dserv.Add(a)
	if err != nil {
		t.Fatal(err)
	}

	// Pin A{}
	err = p.Pin(ctx, a, false)
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
	err = p.Pin(ctx, b, true)
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
	err = p.Pin(ctx, d, true)
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
	err = p.Unpin(ctx, dk, true)
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
	ctx := context.Background()
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	bstore := blockstore.NewBlockstore(dstore)
	bserv := bs.New(bstore, offline.Exchange(bstore))

	dserv := mdag.NewDAGService(bserv)

	// TODO does pinner need to share datastore with blockservice?
	p := NewPinner(dstore, dserv)

	a, _ := randNode()
	_, err := dserv.Add(a)
	if err != nil {
		t.Fatal(err)
	}

	// pin is recursively
	err = p.Pin(ctx, a, true)
	if err != nil {
		t.Fatal(err)
	}

	// pinning directly should fail
	err = p.Pin(ctx, a, false)
	if err == nil {
		t.Fatal("expected direct pin to fail")
	}

	// pinning recursively again should succeed
	err = p.Pin(ctx, a, true)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPinRecursiveFail(t *testing.T) {
	ctx := context.Background()
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	bstore := blockstore.NewBlockstore(dstore)
	bserv := bs.New(bstore, offline.Exchange(bstore))

	dserv := mdag.NewDAGService(bserv)

	p := NewPinner(dstore, dserv)

	a, _ := randNode()
	b, _ := randNode()
	err := a.AddNodeLinkClean("child", b)
	if err != nil {
		t.Fatal(err)
	}

	// Note: this isnt a time based test, we expect the pin to fail
	mctx, cancel := context.WithTimeout(ctx, time.Millisecond)
	defer cancel()
	err = p.Pin(mctx, a, true)
	if err == nil {
		t.Fatal("should have failed to pin here")
	}

	if _, err := dserv.Add(b); err != nil {
		t.Fatal(err)
	}

	// this one is time based... but shouldnt cause any issues
	mctx, cancel = context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := p.Pin(mctx, a, true); err != nil {
		t.Fatal(err)
	}
}
