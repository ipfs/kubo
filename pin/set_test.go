package pin

import (
	"testing"
	"testing/quick"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dssync "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/ipfs/go-ipfs/blocks/blockstore"
	"github.com/ipfs/go-ipfs/blocks/key"
	"github.com/ipfs/go-ipfs/blockservice"
	"github.com/ipfs/go-ipfs/exchange/offline"
	"github.com/ipfs/go-ipfs/merkledag"
)

func ignoreKeys(key.Key) {}

func copyMap(m map[key.Key]uint16) map[key.Key]uint64 {
	c := make(map[key.Key]uint64, len(m))
	for k, v := range m {
		c[k] = uint64(v)
	}
	return c
}

func TestMultisetRoundtrip(t *testing.T) {
	dstore := dssync.MutexWrap(datastore.NewMapDatastore())
	bstore := blockstore.NewBlockstore(dstore)
	bserv := blockservice.New(bstore, offline.Exchange(bstore))
	dag := merkledag.NewDAGService(bserv)

	fn := func(m map[key.Key]uint16) bool {
		// Generate a smaller range for refcounts than full uint64, as
		// otherwise this just becomes overly cpu heavy, splitting it
		// out into too many items. That means we need to convert to
		// the right kind of map. As storeMultiset mutates the map as
		// part of its bookkeeping, this is actually good.
		refcounts := copyMap(m)

		ctx := context.Background()
		n, err := storeMultiset(ctx, dag, refcounts, ignoreKeys)
		if err != nil {
			t.Fatalf("storing multiset: %v", err)
		}
		root := &merkledag.Node{}
		const linkName = "dummylink"
		if err := root.AddNodeLink(linkName, n); err != nil {
			t.Fatalf("adding link to root node: %v", err)
		}

		roundtrip, err := loadMultiset(ctx, dag, root, linkName, ignoreKeys)
		if err != nil {
			t.Fatalf("loading multiset: %v", err)
		}

		orig := copyMap(m)
		success := true
		for k, want := range orig {
			if got, ok := roundtrip[k]; ok {
				if got != want {
					success = false
					t.Logf("refcount changed: %v -> %v for %q", want, got, k)
				}
				delete(orig, k)
				delete(roundtrip, k)
			}
		}
		for k, v := range orig {
			success = false
			t.Logf("refcount missing: %v for %q", v, k)
		}
		for k, v := range roundtrip {
			success = false
			t.Logf("refcount extra: %v for %q", v, k)
		}
		return success
	}
	if err := quick.Check(fn, nil); err != nil {
		t.Fatal(err)
	}
}
