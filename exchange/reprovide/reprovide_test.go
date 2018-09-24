package reprovide_test

import (
	"context"
	"testing"

	testutil "gx/ipfs/QmNfQbgBfARAtrYsBguChX6VJ5nbjeoYy1KdC36aaYWqG8/go-testutil"
	blocks "gx/ipfs/QmRcHuYzAyswytBuMF78rj3LTChYszomRFXNg4685ZN1WM/go-block-format"
	mock "gx/ipfs/QmScZySgru9jaoDa12sSfvh21sWbqF5eXkieTmJzAHJXkQ/go-ipfs-routing/mock"
	ds "gx/ipfs/QmUyz7JTJzgegC6tiJrfby3mPhzcdswVtG4x58TQ6pq8jV/go-datastore"
	dssync "gx/ipfs/QmUyz7JTJzgegC6tiJrfby3mPhzcdswVtG4x58TQ6pq8jV/go-datastore/sync"
	blockstore "gx/ipfs/QmdriVJgKx4JADRgh3cYPXqXmsa1A45SvFki1nDWHhQNtC/go-ipfs-blockstore"
	pstore "gx/ipfs/QmfAQMFpgDU2U4BXG64qVr8HSiictfWvkSBz7Y2oDj65st/go-libp2p-peerstore"

	. "github.com/ipfs/go-ipfs/exchange/reprovide"
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

	keyProvider := NewBlockstoreProvider(bstore)
	reprov := NewReprovider(ctx, clA, keyProvider)
	err := reprov.Reprovide()
	if err != nil {
		t.Fatal(err)
	}

	var providers []pstore.PeerInfo
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
