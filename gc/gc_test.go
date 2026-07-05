package gc

import (
	"context"
	"testing"
	"time"

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
	for range 5 {
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
	for range 5 {
		_, allCids, err := daggen.MakeDagNode(dserv.Add, 5, 2)
		require.NoError(t, err)
		expectedDiscarded = append(expectedDiscarded, toMHs(allCids)...)
	}

	// and some other as "best effort roots"
	var bestEffortRoots []cid.Cid
	for range 5 {
		root, allCids, err := daggen.MakeDagNode(dserv.Add, 5, 2)
		require.NoError(t, err)
		bestEffortRoots = append(bestEffortRoots, root)
		expectedKept = append(expectedKept, toMHs(allCids)...)
	}

	ch := GC(ctx, bs, ds, pinner, func(context.Context) ([]cid.Cid, error) {
		return bestEffortRoots, nil
	})
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

// TestGCSnapshotsBestEffortRootsUnderLock guards the fix for
// https://github.com/ipfs/kubo/issues/10842: GC must collect the best-effort
// roots (the MFS root) while it holds the GC lock, not before. If the roots are
// snapshotted before the lock, a concurrent MFS write, which takes the pin lock,
// can add blocks the snapshot misses and the sweep then deletes them.
func TestGCSnapshotsBestEffortRootsUnderLock(t *testing.T) {
	ctx := context.Background()

	ds := dssync.MutexWrap(datastore.NewMapDatastore())
	bs := blockstore.NewGCBlockstore(blockstore.NewBlockstore(ds), blockstore.NewGCLocker())
	bserv := blockservice.New(bs, offline.Exchange(bs))
	dserv := merkledag.NewDAGService(bserv)
	pinner, err := dspinner.New(ctx, ds, dserv)
	require.NoError(t, err)

	called := false
	roots := func(context.Context) ([]cid.Cid, error) {
		called = true

		// GC holds the exclusive GC lock while this runs, so a pin lock (a
		// shared read lock on the same locker, taken by every MFS mutation)
		// must not be grantable. If it were, the roots would not be snapshotted
		// under the lock and a concurrent write could slip blocks past the sweep.
		probing := make(chan struct{})
		pinLocked := make(chan struct{})
		go func() {
			close(probing)
			unlocker := bs.PinLock(ctx)
			close(pinLocked)
			unlocker.Unlock(ctx)
		}()
		<-probing

		select {
		case <-pinLocked:
			t.Error("pin lock was granted while bestEffortRoots ran: roots are not snapshotted under the GC lock")
		case <-time.After(100 * time.Millisecond):
			// Expected: the held GC lock keeps the pin lock blocked. The probe
			// goroutine unblocks and finishes once GC releases the lock below.
		}
		return nil, nil
	}

	for range GC(ctx, bs, ds, pinner, roots) {
	}
	require.True(t, called, "bestEffortRoots was never called")
}

func toMHs(cids []cid.Cid) []multihash.Multihash {
	res := make([]multihash.Multihash, len(cids))
	for i, c := range cids {
		res[i] = c.Hash()
	}
	return res
}
