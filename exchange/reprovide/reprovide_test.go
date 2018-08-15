package reprovide_test

import (
	"context"
	"testing"

	blocks "gx/ipfs/QmR54CzE4UcdFAZDehj6HFyy3eSHhVsJUpjfnhCmscuStS/go-block-format"
	mock "gx/ipfs/QmRr8DpNhQMzsoqAitUrw43D82pyPXZkyUqarhSAfkrdaQ/go-ipfs-routing/mock"
	ds "gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore"
	dssync "gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore/sync"
	testutil "gx/ipfs/QmXG74iiKQnDstVQq9fPFQEB6JTNSWBbAWE1qsq6L4E5sR/go-testutil"
	blockstore "gx/ipfs/QmYBEfMSquSGnuxBthUoBJNs3F6p4VAPPvAgxq6XXGvTPh/go-ipfs-blockstore"
	pstore "gx/ipfs/QmYLXCWN2myozZpx8Wx4UjrRuQuhY3YtWoMi6SHaXii6aM/go-libp2p-peerstore"

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
