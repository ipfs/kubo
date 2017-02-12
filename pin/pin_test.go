package pin

import (
	"testing"
	"time"

	"github.com/ipfs/go-ipfs/blocks/blockstore"
	bs "github.com/ipfs/go-ipfs/blockservice"
	"github.com/ipfs/go-ipfs/exchange/offline"
	mdag "github.com/ipfs/go-ipfs/merkledag"

	context "context"
	ds "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore"
	dssync "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore/sync"
	"gx/ipfs/Qmb912gdngC1UWwTkhuW8knyRbcWeu5kqkxBpveLmW8bSr/go-ipfs-util"
	cid "gx/ipfs/QmcTcsTvfaeEBRFo1TkFgT8sRmgi1n1LTZpecfVP8fzpGD/go-cid"
)

func randNode() (*mdag.ProtoNode, *cid.Cid) {
	nd := new(mdag.ProtoNode)
	nd.SetData(make([]byte, 32))
	util.NewTimeSeededRand().Read(nd.Data())
	k := nd.Cid()
	return nd, k
}

func assertPinned(t *testing.T, p Pinner, c *cid.Cid, failmsg string) {
	_, pinned, err := p.IsPinned(c)
	if err != nil {
		t.Fatal(err)
	}

	if !pinned {
		t.Fatal(failmsg)
	}
}

func TestPinnerBasic(t *testing.T) {
	ctx := context.Background()

	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	bstore := blockstore.NewBlockstore(dstore)
	bserv := bs.New(bstore, offline.Exchange(bstore))

	dserv := mdag.NewDAGService(bserv)

	// TODO does pinner need to share datastore with blockservice?
	p := NewPinner(dstore, dserv, dserv)

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

	assertPinned(t, p, ak, "Failed to find key")

	// create new node c, to be indirectly pinned through b
	c, _ := randNode()
	ck, err := dserv.Add(c)
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

	assertPinned(t, p, ck, "child of recursively pinned node not found")

	bk := b.Cid()
	assertPinned(t, p, bk, "Recursively pinned node not found..")

	d, _ := randNode()
	d.AddNodeLink("a", a)
	d.AddNodeLink("c", c)

	e, _ := randNode()
	d.AddNodeLink("e", e)

	// Must be in dagserv for unpin to work
	_, err = dserv.Add(e)
	if err != nil {
		t.Fatal(err)
	}
	_, err = dserv.Add(d)
	if err != nil {
		t.Fatal(err)
	}

	// Add D{A,C,E}
	err = p.Pin(ctx, d, true)
	if err != nil {
		t.Fatal(err)
	}

	dk := d.Cid()
	assertPinned(t, p, dk, "pinned node not found.")

	// Test recursive unpin
	err = p.Unpin(ctx, dk, true)
	if err != nil {
		t.Fatal(err)
	}

	err = p.Flush()
	if err != nil {
		t.Fatal(err)
	}

	np, err := LoadPinner(dstore, dserv, dserv)
	if err != nil {
		t.Fatal(err)
	}

	// Test directly pinned
	assertPinned(t, np, ak, "Could not find pinned node!")

	// Test recursively pinned
	assertPinned(t, np, bk, "could not find recursively pinned node")
}

func TestDuplicateSemantics(t *testing.T) {
	ctx := context.Background()
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	bstore := blockstore.NewBlockstore(dstore)
	bserv := bs.New(bstore, offline.Exchange(bstore))

	dserv := mdag.NewDAGService(bserv)

	// TODO does pinner need to share datastore with blockservice?
	p := NewPinner(dstore, dserv, dserv)

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

func TestFlush(t *testing.T) {
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	bstore := blockstore.NewBlockstore(dstore)
	bserv := bs.New(bstore, offline.Exchange(bstore))

	dserv := mdag.NewDAGService(bserv)
	p := NewPinner(dstore, dserv, dserv)
	_, k := randNode()

	p.PinWithMode(k, Recursive)
	if err := p.Flush(); err != nil {
		t.Fatal(err)
	}
	assertPinned(t, p, k, "expected key to still be pinned")
}

func TestPinRecursiveFail(t *testing.T) {
	ctx := context.Background()
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	bstore := blockstore.NewBlockstore(dstore)
	bserv := bs.New(bstore, offline.Exchange(bstore))
	dserv := mdag.NewDAGService(bserv)

	p := NewPinner(dstore, dserv, dserv)

	a, _ := randNode()
	b, _ := randNode()
	err := a.AddNodeLinkClean("child", b)
	if err != nil {
		t.Fatal(err)
	}

	// NOTE: This isnt a time based test, we expect the pin to fail
	mctx, _ := context.WithTimeout(ctx, time.Millisecond)
	err = p.Pin(mctx, a, true)
	if err == nil {
		t.Fatal("should have failed to pin here")
	}

	_, err = dserv.Add(b)
	if err != nil {
		t.Fatal(err)
	}

	_, err = dserv.Add(a)
	if err != nil {
		t.Fatal(err)
	}

	// this one is time based... but shouldnt cause any issues
	mctx, _ = context.WithTimeout(ctx, time.Second)
	err = p.Pin(mctx, a, true)
	if err != nil {
		t.Fatal(err)
	}
}
