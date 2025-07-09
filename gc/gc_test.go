package gc

import (
	"context"
	"testing"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange/offline"
	"github.com/ipfs/boxo/ipld/merkledag"
	mdutils "github.com/ipfs/boxo/ipld/merkledag/test"
	pin "github.com/ipfs/boxo/pinning/pinner"
	"github.com/ipfs/boxo/pinning/pinner/dspinner"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"
)

func TestGC(t *testing.T) {
	ctx := context.Background()

	ds := dssync.MutexWrap(datastore.NewMapDatastore())
	bs := blockstore.NewGCBlockstore(blockstore.NewBlockstore(ds), blockstore.NewGCLocker())
	bserv := blockservice.New(bs, offline.Exchange(bs))
	dserv := merkledag.NewDAGService(bserv)
	pinner, err := dspinner.New(ctx, ds, dserv)
	require.NoError(t, err)

	daggen := mdutils.NewDAGGenerator()

	var expectedKept []multihash.Multihash
	var expectedDiscarded []multihash.Multihash

	// add some pins
	for i := 0; i < 5; i++ {
		// direct
		root, _, err := daggen.MakeDagNode(dserv.Add, 0, 1)
		require.NoError(t, err)
		err = pinner.PinWithMode(ctx, root, pin.Direct, "")
		require.NoError(t, err)
		expectedKept = append(expectedKept, root.Hash())

		// recursive
		root, allCids, err := daggen.MakeDagNode(dserv.Add, 5, 2)
		require.NoError(t, err)
		err = pinner.PinWithMode(ctx, root, pin.Recursive, "")
		require.NoError(t, err)
		expectedKept = append(expectedKept, toMHs(allCids)...)
	}

	err = pinner.Flush(ctx)
	require.NoError(t, err)

	// add more dags to be GCed
	for i := 0; i < 5; i++ {
		_, allCids, err := daggen.MakeDagNode(dserv.Add, 5, 2)
		require.NoError(t, err)
		expectedDiscarded = append(expectedDiscarded, toMHs(allCids)...)
	}

	// and some other as "best effort roots"
	var bestEffortRoots []cid.Cid
	for i := 0; i < 5; i++ {
		root, allCids, err := daggen.MakeDagNode(dserv.Add, 5, 2)
		require.NoError(t, err)
		bestEffortRoots = append(bestEffortRoots, root)
		expectedKept = append(expectedKept, toMHs(allCids)...)
	}

	ch := GC(ctx, bs, ds, pinner, bestEffortRoots)
	var discarded []multihash.Multihash
	for res := range ch {
		require.NoError(t, res.Error)
		discarded = append(discarded, res.KeyRemoved.Hash())
	}

	allKeys, err := bs.AllKeysChan(ctx)
	require.NoError(t, err)
	var kept []multihash.Multihash
	for key := range allKeys {
		kept = append(kept, key.Hash())
	}

	require.ElementsMatch(t, expectedDiscarded, discarded)
	require.ElementsMatch(t, expectedKept, kept)
}

func toMHs(cids []cid.Cid) []multihash.Multihash {
	res := make([]multihash.Multihash, len(cids))
	for i, c := range cids {
		res[i] = c.Hash()
	}
	return res
}
