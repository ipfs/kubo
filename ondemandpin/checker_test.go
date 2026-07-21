package ondemandpin

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/kubo/config"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeClock struct{ t atomic.Int64 }

func newFakeClock() *fakeClock {
	c := &fakeClock{}
	c.t.Store(time.Now().UnixNano())
	return c
}

func (c *fakeClock) Now() time.Time          { return time.Unix(0, c.t.Load()) }
func (c *fakeClock) Advance(d time.Duration) { c.t.Add(int64(d)) }

type mockRouting struct {
	mu        sync.Mutex
	providers map[cid.Cid][]peer.AddrInfo
}

func newMockRouting() *mockRouting {
	return &mockRouting{providers: make(map[cid.Cid][]peer.AddrInfo)}
}

func (m *mockRouting) setProviders(c cid.Cid, peerIDs ...peer.ID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	infos := make([]peer.AddrInfo, len(peerIDs))
	for i, pid := range peerIDs {
		infos[i] = peer.AddrInfo{ID: pid}
	}
	m.providers[c] = infos
}

func (m *mockRouting) FindProvidersAsync(ctx context.Context, c cid.Cid, limit int) <-chan peer.AddrInfo {
	ch := make(chan peer.AddrInfo)
	go func() {
		defer close(ch)
		m.mu.Lock()
		provs := m.providers[c]
		m.mu.Unlock()
		for i, pi := range provs {
			if i >= limit {
				break
			}
			select {
			case ch <- pi:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

func (m *mockRouting) Provide(context.Context, cid.Cid, bool) error { return nil }

type mockPins struct {
	pinned map[cid.Cid]string
}

func newMockPins() *mockPins { return &mockPins{pinned: make(map[cid.Cid]string)} }

func (m *mockPins) Pin(_ context.Context, c cid.Cid, name string) error {
	m.pinned[c] = name
	return nil
}

func (m *mockPins) Unpin(_ context.Context, c cid.Cid) error {
	delete(m.pinned, c)
	return nil
}

func (m *mockPins) IsPinned(_ context.Context, c cid.Cid) (bool, error) {
	_, ok := m.pinned[c]
	return ok, nil
}

func (m *mockPins) HasPinWithName(_ context.Context, c cid.Cid, name string) (bool, error) {
	n, ok := m.pinned[c]
	return ok && n == name, nil
}

func (m *mockPins) isPinned(c cid.Cid) bool {
	_, ok := m.pinned[c]
	return ok
}

func newTestChecker(t *testing.T) (*Checker, *Store, *mockRouting, *mockPins, *fakeClock) {
	t.Helper()
	store := NewStore(dssync.MutexWrap(datastore.NewMapDatastore()))
	r := newMockRouting()
	p := newMockPins()
	clock := newFakeClock()

	checker := NewChecker(store, p, nil, r, peer.ID("self"), config.OnDemandPinning{})
	checker.checkInterval = time.Minute
	checker.unpinGracePeriod = 200 * time.Millisecond
	checker.now = clock.Now

	return checker, store, r, p, clock
}

// Under-replicated content gets pinned.
func TestCheckerPinsBelowTarget(t *testing.T) {
	ctx := context.Background()
	checker, store, r, p, _ := newTestChecker(t)
	c := testCID(t, "under-replicated")

	require.NoError(t, store.Add(ctx, c))
	r.setProviders(c, peer.ID("p1"), peer.ID("p2"))

	checker.checkAll(ctx)

	assert.True(t, p.isPinned(c))
}

// Well-replicated content is left alone.
func TestCheckerDoesNotPinAboveTarget(t *testing.T) {
	ctx := context.Background()
	checker, store, r, p, _ := newTestChecker(t)
	c := testCID(t, "well-replicated")

	require.NoError(t, store.Add(ctx, c))
	r.setProviders(c, peer.ID("p1"), peer.ID("p2"), peer.ID("p3"), peer.ID("p4"), peer.ID("p5"), peer.ID("p6"))

	checker.checkAll(ctx)

	assert.False(t, p.isPinned(c))
}

// Pinned content is unpinned only after the grace period expires.
func TestCheckerUnpinsAfterGracePeriod(t *testing.T) {
	ctx := context.Background()
	checker, store, r, p, clock := newTestChecker(t)
	c := testCID(t, "recovering")

	require.NoError(t, store.Add(ctx, c))
	r.setProviders(c, peer.ID("p1"))
	checker.checkAll(ctx)
	require.True(t, p.isPinned(c))

	// Providers recover above target.
	r.setProviders(c, peer.ID("p1"), peer.ID("p2"), peer.ID("p3"), peer.ID("p4"), peer.ID("p5"), peer.ID("p6"))
	checker.checkAll(ctx)
	assert.True(t, p.isPinned(c), "not yet past grace period")

	clock.Advance(250 * time.Millisecond)
	checker.checkAll(ctx)
	assert.False(t, p.isPinned(c), "past grace period")
}

type pinDuringLookupRouting struct {
	*mockRouting
	pins    *mockPins
	pinName string
}

func (r *pinDuringLookupRouting) FindProvidersAsync(ctx context.Context, c cid.Cid, limit int) <-chan peer.AddrInfo {
	r.pins.pinned[c] = r.pinName
	return r.mockRouting.FindProvidersAsync(ctx, c, limit)
}

// Re-check before Pin: a user pin that landed during the DHT lookup must not be overwritten.
func TestCheckerSkipsPinCreatedDuringLookup(t *testing.T) {
	ctx := context.Background()
	store := NewStore(dssync.MutexWrap(datastore.NewMapDatastore()))
	r := newMockRouting()
	p := newMockPins()
	racing := &pinDuringLookupRouting{mockRouting: r, pins: p, pinName: "user-pin"}
	checker := NewChecker(store, p, nil, racing, peer.ID("self"), config.OnDemandPinning{})
	checker.now = newFakeClock().Now

	c := testCID(t, "raced")
	require.NoError(t, store.Add(ctx, c))
	r.setProviders(c, peer.ID("p1"))

	checker.checkAll(ctx)

	assert.Equal(t, "user-pin", p.pinned[c])
}

// A pin with the reserved name is managed even if the store never recorded a
// separate ownership flag (the old crash window between Pin and saveRecord).
func TestCheckerOwnsPinByNameNotStoreField(t *testing.T) {
	ctx := context.Background()
	checker, store, r, p, clock := newTestChecker(t)
	c := testCID(t, "name-owned")

	require.NoError(t, store.Add(ctx, c))
	require.NoError(t, p.Pin(ctx, c, OnDemandPinName))
	r.setProviders(c, peer.ID("p1"), peer.ID("p2"), peer.ID("p3"), peer.ID("p4"), peer.ID("p5"), peer.ID("p6"))

	checker.checkAll(ctx)
	assert.True(t, p.isPinned(c), "grace period just started")

	clock.Advance(250 * time.Millisecond)
	checker.checkAll(ctx)
	assert.False(t, p.isPinned(c), "name-owned pin must unpin after grace")
}

// blockingRouting closes only when ctx is cancelled, mimicking a hung lookup.
type blockingRouting struct{}

func (blockingRouting) FindProvidersAsync(ctx context.Context, _ cid.Cid, _ int) <-chan peer.AddrInfo {
	ch := make(chan peer.AddrInfo)
	go func() {
		defer close(ch)
		<-ctx.Done()
	}()
	return ch
}

func (blockingRouting) Provide(context.Context, cid.Cid, bool) error { return nil }

func TestCountProvidersUnknownOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	count, ok := CountProviders(ctx, blockingRouting{}, peer.ID("self"), testCID(t, "unknown"), 5)
	assert.False(t, ok)
	assert.Equal(t, 0, count)
}

// emitThenBlockRouting emits providers, then blocks until ctx cancel.
type emitThenBlockRouting struct {
	providers []peer.ID
}

func (r emitThenBlockRouting) FindProvidersAsync(ctx context.Context, _ cid.Cid, limit int) <-chan peer.AddrInfo {
	ch := make(chan peer.AddrInfo)
	go func() {
		defer close(ch)
		for i, id := range r.providers {
			if i >= limit {
				break
			}
			select {
			case ch <- peer.AddrInfo{ID: id}:
			case <-ctx.Done():
				return
			}
		}
		<-ctx.Done()
	}()
	return ch
}

func (emitThenBlockRouting) Provide(context.Context, cid.Cid, bool) error { return nil }

func TestCountProvidersOkWhenEnoughFoundDespiteCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := emitThenBlockRouting{providers: []peer.ID{"p1", "p2", "p3", "p4", "p5"}}

	done := make(chan struct{})
	var count int
	var ok bool
	go func() {
		defer close(done)
		count, ok = CountProviders(ctx, r, peer.ID("self"), testCID(t, "enough"), 5)
	}()

	time.Sleep(20 * time.Millisecond) // let providers flush
	cancel()
	<-done

	require.True(t, ok, "count >= target is reliable even if the lookup is then cancelled")
	assert.Equal(t, 5, count)
}

// A cancelled/timed-out provider lookup must not be treated as zero providers.
func TestCheckerSkipsWhenProviderCountUnknown(t *testing.T) {
	ctx := context.Background()
	store := NewStore(dssync.MutexWrap(datastore.NewMapDatastore()))
	p := newMockPins()
	checker := NewChecker(store, p, nil, blockingRouting{}, peer.ID("self"), config.OnDemandPinning{})
	checker.now = newFakeClock().Now

	c := testCID(t, "hung-lookup")
	require.NoError(t, store.Add(ctx, c))

	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	checker.checkRecord(checkCtx, mustGet(t, store, c))

	assert.False(t, p.isPinned(c))
}

func mustGet(t *testing.T, store *Store, c cid.Cid) *Record {
	t.Helper()
	rec, err := store.Get(context.Background(), c)
	require.NoError(t, err)
	return rec
}
