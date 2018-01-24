package reprovide_test

import (
	"context"
	"testing"

	blockstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	mock "github.com/ipfs/go-ipfs/routing/mock"
	ds "gx/ipfs/QmPpegoMqhAEqjncrzArm7KVWAkCm78rqL2DPuNjhPrshg/go-datastore"
	dssync "gx/ipfs/QmPpegoMqhAEqjncrzArm7KVWAkCm78rqL2DPuNjhPrshg/go-datastore/sync"
	pstore "gx/ipfs/QmeZVQzUrXqaszo24DAoHfGzcmCptN9JyngLkGAiEfk2x7/go-libp2p-peerstore"
	blocks "gx/ipfs/Qmej7nf81hi2x2tvjRBF3mcp74sQyuDH4VMYDGd1YtXjb2/go-block-format"
	testutil "gx/ipfs/QmfB65MYJqaKzBiMvW47fquCRhmEeXW6AhrJSGM7TeY5eG/go-testutil"

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
