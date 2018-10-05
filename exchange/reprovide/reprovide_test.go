package reprovide_test

import (
	"context"
	"testing"

	testutil "gx/ipfs/QmNfQbgBfARAtrYsBguChX6VJ5nbjeoYy1KdC36aaYWqG8/go-testutil"
	blocks "gx/ipfs/QmRcHuYzAyswytBuMF78rj3LTChYszomRFXNg4685ZN1WM/go-block-format"
	pstore "gx/ipfs/QmSJ36wcYQyEViJUWUEhJU81tw1KdakTKqLLHbvYbA9zDv/go-libp2p-peerstore"
	mock "gx/ipfs/QmYKPBQpSSWwmgNTvVE3vQdPoeqxwudPQnXJ4hU383RsSA/go-ipfs-routing/mock"
	ds "gx/ipfs/QmbQshXLNcCPRUGZv4sBGxnZNAHREA6MKeomkwihNXPZWP/go-datastore"
	dssync "gx/ipfs/QmbQshXLNcCPRUGZv4sBGxnZNAHREA6MKeomkwihNXPZWP/go-datastore/sync"
	blockstore "gx/ipfs/QmfUhZX9KpvJiuiziUzP2cjhRAyqHJURsPgRKn1cdDZMKa/go-ipfs-blockstore"

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
