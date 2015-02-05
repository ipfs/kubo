package reprovide_test

import (
	"testing"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dssync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"

	mock "github.com/jbenet/go-ipfs/routing/mock"
	blocks "github.com/jbenet/go-ipfs/struct/blocks"
	blockstore "github.com/jbenet/go-ipfs/struct/blocks/blockstore"
	testutil "github.com/jbenet/go-ipfs/util/testutil"

	. "github.com/jbenet/go-ipfs/exchange/reprovide"
)

func TestReprovide(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mrserv := mock.NewServer()

	idA := testutil.RandIdentityOrFatal(t)
	idB := testutil.RandIdentityOrFatal(t)

	clA := mrserv.Client(idA)
	clB := mrserv.Client(idB)

	bstore := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))

	blk := blocks.NewBlock([]byte("this is a test"))
	bstore.Put(blk)

	reprov := NewReprovider(clA, bstore)
	err := reprov.Reprovide(ctx)
	if err != nil {
		t.Fatal(err)
	}

	provs, err := clB.FindProviders(ctx, blk.Key())
	if err != nil {
		t.Fatal(err)
	}

	if len(provs) == 0 {
		t.Fatal("Should have gotten a provider")
	}

	if provs[0].ID != idA.ID() {
		t.Fatal("Somehow got the wrong peer back as a provider.")
	}
}
