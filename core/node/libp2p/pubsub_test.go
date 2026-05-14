package libp2p

import (
	"encoding/binary"
	"testing"

	"github.com/ipfs/go-datastore"
	syncds "github.com/ipfs/go-datastore/sync"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

// TestSeqnoStore tests the seqnoStore implementation which backs the
// BasicSeqnoValidator. The validator prevents message cycles when network
// diameter exceeds the timecache span by tracking the maximum sequence number
// seen from each peer.
func TestSeqnoStore(t *testing.T) {
	ctx := t.Context()
	ds := syncds.MutexWrap(datastore.NewMapDatastore())
	store := &seqnoStore{ds: ds}

	peerA, err := peer.Decode("12D3KooWGC6TvWhfapngX6wvJHMYvKpDMXPb3ZnCZ6dMoaMtimQ5")
	require.NoError(t, err)
	peerB, err := peer.Decode("12D3KooWJRqDKTRjvXeGdUEgwkHNsoghYMBUagNYgLPdA4mqdTeo")
	require.NoError(t, err)

	// BasicSeqnoValidator expects Get to return (nil, nil) for unknown peers,
	// not an error. This allows the validator to accept the first message from
	// any peer without special-casing.
	t.Run("unknown peer returns nil without error", func(t *testing.T) {
		val, err := store.Get(ctx, peerA)
		require.NoError(t, err)
		require.Nil(t, val, "unknown peer should return nil, not empty slice")
	})

	// Verify basic store/retrieve functionality with a sequence number encoded
	// as big-endian uint64, matching the format used by BasicSeqnoValidator.
	t.Run("stores and retrieves seqno", func(t *testing.T) {
		seqno := uint64(12345)
		data := make([]byte, 8)
		binary.BigEndian.PutUint64(data, seqno)

		err := store.Put(ctx, peerA, data)
		require.NoError(t, err)

		val, err := store.Get(ctx, peerA)
		require.NoError(t, err)
		require.Equal(t, seqno, binary.BigEndian.Uint64(val))
	})

	// Each peer must have isolated storage. If peer data leaked between peers,
	// the validator would incorrectly reject valid messages or accept replays.
	t.Run("isolates seqno per peer", func(t *testing.T) {
		seqnoA := uint64(100)
		seqnoB := uint64(200)
		dataA := make([]byte, 8)
		dataB := make([]byte, 8)
		binary.BigEndian.PutUint64(dataA, seqnoA)
		binary.BigEndian.PutUint64(dataB, seqnoB)

		err := store.Put(ctx, peerA, dataA)
		require.NoError(t, err)
		err = store.Put(ctx, peerB, dataB)
		require.NoError(t, err)

		valA, err := store.Get(ctx, peerA)
		require.NoError(t, err)
		require.Equal(t, seqnoA, binary.BigEndian.Uint64(valA))

		valB, err := store.Get(ctx, peerB)
		require.NoError(t, err)
		require.Equal(t, seqnoB, binary.BigEndian.Uint64(valB))
	})

	// The validator updates the stored seqno when accepting messages with
	// higher seqnos. This test verifies that updates work correctly.
	t.Run("updates seqno to higher value", func(t *testing.T) {
		seqno1 := uint64(1000)
		seqno2 := uint64(2000)
		data1 := make([]byte, 8)
		data2 := make([]byte, 8)
		binary.BigEndian.PutUint64(data1, seqno1)
		binary.BigEndian.PutUint64(data2, seqno2)

		err := store.Put(ctx, peerA, data1)
		require.NoError(t, err)

		err = store.Put(ctx, peerA, data2)
		require.NoError(t, err)

		val, err := store.Get(ctx, peerA)
		require.NoError(t, err)
		require.Equal(t, seqno2, binary.BigEndian.Uint64(val))
	})

	// Verify the datastore key format. This is important for:
	// 1. Debugging: operators can inspect/clear pubsub state
	// 2. Migrations: future changes need to know the key format
	t.Run("uses expected datastore key format", func(t *testing.T) {
		seqno := uint64(42)
		data := make([]byte, 8)
		binary.BigEndian.PutUint64(data, seqno)

		err := store.Put(ctx, peerA, data)
		require.NoError(t, err)

		// Verify we can read directly from datastore with expected key
		expectedKey := datastore.NewKey("/pubsub/seqno/" + peerA.String())
		val, err := ds.Get(ctx, expectedKey)
		require.NoError(t, err)
		require.Equal(t, seqno, binary.BigEndian.Uint64(val))
	})

	// Verify data persists when creating a new store instance with the same
	// underlying datastore. This simulates node restart.
	t.Run("persists across store instances", func(t *testing.T) {
		seqno := uint64(99999)
		data := make([]byte, 8)
		binary.BigEndian.PutUint64(data, seqno)

		err := store.Put(ctx, peerB, data)
		require.NoError(t, err)

		// Create new store instance with same datastore
		store2 := &seqnoStore{ds: ds}
		val, err := store2.Get(ctx, peerB)
		require.NoError(t, err)
		require.Equal(t, seqno, binary.BigEndian.Uint64(val))
	})
}
