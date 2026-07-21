package ondemandpin

import (
	"context"
	"errors"
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
	checker.graceJitter = func() time.Duration { return 0 }

	return checker, store, r, p, clock
}

func providers(n int) []peer.ID {
	out := make([]peer.ID, n)
	for i := range out {
		out[i] = peer.ID(string(rune('a' + i)))
	}
	return out
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

func TestCheckerDoesNotPinInDeadband(t *testing.T) {
	ctx := context.Background()
	checker, store, r, p, _ := newTestChecker(t)
	c := testCID(t, "deadband")

	require.NoError(t, store.Add(ctx, c))
	r.setProviders(c, providers(6)...) // min=5, max=7

	checker.checkAll(ctx)

	assert.False(t, p.isPinned(c))
}

func TestCheckerUnpinsAfterGracePeriod(t *testing.T) {
	ctx := context.Background()
	checker, store, r, p, clock := newTestChecker(t)
	c := testCID(t, "recovering")

	require.NoError(t, store.Add(ctx, c))
	r.setProviders(c, peer.ID("p1"))
	checker.checkAll(ctx)
	require.True(t, p.isPinned(c))

	// Providers recover above max (default 7).
	r.setProviders(c, providers(8)...)
	checker.checkAll(ctx)
	assert.True(t, p.isPinned(c), "not yet past grace period")

	clock.Advance(250 * time.Millisecond)
	checker.checkAll(ctx)
	assert.False(t, p.isPinned(c), "past grace period")
}

func TestCheckerGraceIncludesJitter(t *testing.T) {
	ctx := context.Background()
	checker, store, r, p, clock := newTestChecker(t)
	checker.graceJitter = func() time.Duration { return 100 * time.Millisecond }
	c := testCID(t, "jitter")

	require.NoError(t, store.Add(ctx, c))
	require.NoError(t, p.Pin(ctx, c, OnDemandPinName))
	r.setProviders(c, providers(8)...)

	checker.checkAll(ctx)
	assert.True(t, p.isPinned(c))

	clock.Advance(250 * time.Millisecond) // grace only; jitter not yet elapsed
	checker.checkAll(ctx)
	assert.True(t, p.isPinned(c), "still within grace+jitter")

	clock.Advance(100 * time.Millisecond)
	checker.checkAll(ctx)
	assert.False(t, p.isPinned(c), "past grace+jitter")
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

// Pin that appeared during the DHT lookup is left alone.
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

func TestCheckerOwnsPinByNameNotStoreField(t *testing.T) {
	ctx := context.Background()
	checker, store, r, p, clock := newTestChecker(t)
	c := testCID(t, "name-owned")

	require.NoError(t, store.Add(ctx, c))
	require.NoError(t, p.Pin(ctx, c, OnDemandPinName))
	r.setProviders(c, providers(8)...)

	checker.checkAll(ctx)
	assert.True(t, p.isPinned(c), "grace period just started")

	clock.Advance(250 * time.Millisecond)
	checker.checkAll(ctx)
	assert.False(t, p.isPinned(c), "name-owned pin must unpin after grace")
}

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

	count, ok := CountProviders(ctx, blockingRouting{}, peer.ID("self"), testCID(t, "unknown"), 5, 7)
	assert.False(t, ok)
	assert.Equal(t, 0, count)
}

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
		count, ok = CountProviders(ctx, r, peer.ID("self"), testCID(t, "enough"), 5, 7)
	}()

	time.Sleep(20 * time.Millisecond) // let providers flush
	cancel()
	<-done

	require.True(t, ok)
	assert.Equal(t, 5, count)
}

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
	checker.checkRecord(checkCtx, mustGet(t, store, c), false)

	assert.False(t, p.isPinned(c))
	rec := mustGet(t, store, c)
	assert.Equal(t, 1, rec.FailureCount)
	assert.False(t, rec.NextCheckAt.IsZero())
}

type failPinOnce struct {
	*mockPins
	fail bool
}

func (p *failPinOnce) Pin(ctx context.Context, c cid.Cid, name string) error {
	if p.fail {
		return errors.New("pin failed")
	}
	return p.mockPins.Pin(ctx, c, name)
}

type capturePinCtx struct {
	*mockPins
	pinCtx context.Context
}

func (p *capturePinCtx) Pin(ctx context.Context, c cid.Cid, name string) error {
	p.pinCtx = ctx
	return p.mockPins.Pin(ctx, c, name)
}

func TestPinContextHasNoCheckTimeout(t *testing.T) {
	ctx := context.Background()
	store := NewStore(dssync.MutexWrap(datastore.NewMapDatastore()))
	r := newMockRouting()
	pins := &capturePinCtx{mockPins: newMockPins()}
	checker := NewChecker(store, pins, nil, r, peer.ID("self"), config.OnDemandPinning{})
	checker.now = newFakeClock().Now
	checker.graceJitter = func() time.Duration { return 0 }

	c := testCID(t, "pin-ctx")
	require.NoError(t, store.Add(ctx, c))
	r.setProviders(c, peer.ID("p1"))

	checker.checkAll(ctx)
	require.NotNil(t, pins.pinCtx)
	_, hasDeadline := pins.pinCtx.Deadline()
	assert.False(t, hasDeadline)
}

func TestCheckerBackoffSkipsUntilDue(t *testing.T) {
	ctx := context.Background()
	store := NewStore(dssync.MutexWrap(datastore.NewMapDatastore()))
	r := newMockRouting()
	pins := &failPinOnce{mockPins: newMockPins(), fail: true}
	clock := newFakeClock()
	checker := NewChecker(store, pins, nil, r, peer.ID("self"), config.OnDemandPinning{})
	checker.checkInterval = time.Minute
	checker.now = clock.Now
	checker.graceJitter = func() time.Duration { return 0 }

	c := testCID(t, "backoff")
	require.NoError(t, store.Add(ctx, c))
	r.setProviders(c, peer.ID("p1"))

	checker.checkAll(ctx)
	assert.False(t, pins.isPinned(c))
	rec := mustGet(t, store, c)
	require.Equal(t, 1, rec.FailureCount)
	require.Equal(t, clock.Now().Add(time.Minute), rec.NextCheckAt)

	pins.fail = false
	checker.checkAll(ctx)
	assert.False(t, pins.isPinned(c))

	clock.Advance(time.Minute)
	checker.checkAll(ctx)
	assert.True(t, pins.isPinned(c))
	rec = mustGet(t, store, c)
	assert.Equal(t, 0, rec.FailureCount)
	assert.True(t, rec.NextCheckAt.IsZero())
}

func mustGet(t *testing.T, store *Store, c cid.Cid) *Record {
	t.Helper()
	rec, err := store.Get(context.Background(), c)
	require.NoError(t, err)
	return rec
}
