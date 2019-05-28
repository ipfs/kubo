package simple_test

import (
	"context"
	"testing"
	"time"

	blocks "github.com/ipfs/go-block-format"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/go-ipfs-blockstore"
	mock "github.com/ipfs/go-ipfs-routing/mock"
	peer "github.com/libp2p/go-libp2p-core/peer"
	testutil "github.com/libp2p/go-libp2p-testing/net"

	. "github.com/ipfs/go-ipfs/provider/simple"
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
	err := bstore.Put(blk)
	if err != nil {
		t.Fatal(err)
	}

	keyProvider := NewBlockstoreProvider(bstore)
	reprov := NewReprovider(ctx, time.Hour, clA, keyProvider)
	err = reprov.Reprovide()
	if err != nil {
		t.Fatal(err)
	}

	var providers []peer.AddrInfo
	maxProvs := 100

	provChan := clB.FindProvidersAsync(ctx, blk.Cid(), maxProvs)
	for p := range provChan {
		providers = append(providers, p)
	}

	if len(providers) == 0 {
		t.Fatal("Should have gotten a provider")
	}

	if providers[0].ID != idA.ID() {
		t.Fatal("Somehow got the wrong peer back as a provider.")
	}
}
