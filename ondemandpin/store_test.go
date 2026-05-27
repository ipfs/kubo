package ondemandpin

import (
	"context"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	mh "github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCID(t *testing.T, data string) cid.Cid {
	t.Helper()
	h, err := mh.Sum([]byte(data), mh.SHA2_256, -1)
	require.NoError(t, err)
	return cid.NewCidV1(cid.Raw, h)
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(dssync.MutexWrap(datastore.NewMapDatastore()))
}

// Add, Get, Remove lifecycle works.
func TestStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	c := testCID(t, "hello")

	require.NoError(t, s.Add(ctx, c))

	rec, err := s.Get(ctx, c)
	require.NoError(t, err)
	assert.Equal(t, c, rec.Cid)
	assert.False(t, rec.PinnedByUs)

	require.NoError(t, s.Remove(ctx, c))

	_, err = s.Get(ctx, c)
	assert.Error(t, err)
}

// List returns all registered records.
func TestStoreList(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	require.NoError(t, s.Add(ctx, testCID(t, "a")))
	require.NoError(t, s.Add(ctx, testCID(t, "b")))

	records, err := s.List(ctx)
	require.NoError(t, err)
	assert.Len(t, records, 2)
}
