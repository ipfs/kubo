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
