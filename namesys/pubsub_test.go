package namesys

import (
	"context"
	"sync"
	"testing"
	"time"

	path "github.com/ipfs/go-ipfs/path"
	mockrouting "github.com/ipfs/go-ipfs/routing/mock"

	floodsub "gx/ipfs/QmP1T1SGU6276R2MHKP2owbck37Fnzd6ZkpyNJvnG2LoTG/go-libp2p-floodsub"
	p2phost "gx/ipfs/QmP46LGWhzVZTMmt5akNNLfoV8qL4h5wTwmzQxLyDafggd/go-libp2p-host"
	routing "gx/ipfs/QmPCGUjMRuBcPybZFpjhzpifwPP9wPRoiy5geTQKU4vqWA/go-libp2p-routing"
	peer "gx/ipfs/QmWNY7dV54ZDYmTA1ykVdwNCqC11mpU4zSUp6XDpLTH9eG/go-libp2p-peer"
	pstore "gx/ipfs/QmYijbtjCxFEjSXaudaQAUz3LN5VKLssm8WCUsRoqzXmQR/go-libp2p-peerstore"
	bhost "gx/ipfs/QmYmhgAcvmDGXct1qBvc1kz9BxQSit1XBrTeiGZp2FvRyn/go-libp2p-blankhost"
	netutil "gx/ipfs/QmZTcPxK6VqrwY94JpKZPvEqAZ6tEr1rLrpcqJbbRZbg2V/go-libp2p-netutil"
	ci "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	ds "gx/ipfs/QmdHG8MAuARdGHxx4rPQASLcvhz24fzjSQq7AJRAQEorq5/go-datastore"
	testutil "gx/ipfs/QmeDA8gNhvRTsbrjEieay5wezupJDiky8xvCzDABbsGzmp/go-testutil"
)

func newNetHost(ctx context.Context, t *testing.T) p2phost.Host {
	netw := netutil.GenSwarmNetwork(t, ctx)
	return bhost.NewBlankHost(netw)
}

func newNetHosts(ctx context.Context, t *testing.T, n int) []p2phost.Host {
	var out []p2phost.Host

	for i := 0; i < n; i++ {
		h := newNetHost(ctx, t)
		out = append(out, h)
	}

	return out
}

// PubKeyFetcher implementation with a global key store
type mockKeyStore struct {
	keys map[peer.ID]ci.PubKey
	mx   sync.Mutex
}

func (m *mockKeyStore) addPubKey(id peer.ID, pkey ci.PubKey) {
	m.mx.Lock()
	defer m.mx.Unlock()
	m.keys[id] = pkey
}

func (m *mockKeyStore) getPubKey(id peer.ID) (ci.PubKey, error) {
	m.mx.Lock()
	defer m.mx.Unlock()
	pkey, ok := m.keys[id]
	if ok {
		return pkey, nil
	}

	return nil, routing.ErrNotFound
}

func (m *mockKeyStore) GetPublicKey(ctx context.Context, id peer.ID) (ci.PubKey, error) {
	return m.getPubKey(id)
}

func newMockKeyStore() *mockKeyStore {
	return &mockKeyStore{
		keys: make(map[peer.ID]ci.PubKey),
	}
}

// ConentRouting mock
func newMockRouting(ms mockrouting.Server, ks *mockKeyStore, host p2phost.Host) routing.ContentRouting {
	id := host.ID()

	privk := host.Peerstore().PrivKey(id)
	pubk := host.Peerstore().PubKey(id)
	pi := host.Peerstore().PeerInfo(id)

	ks.addPubKey(id, pubk)
	return ms.Client(testutil.NewIdentity(id, pi.Addrs[0], privk, pubk))
}

func newMockRoutingForHosts(ms mockrouting.Server, ks *mockKeyStore, hosts []p2phost.Host) []routing.ContentRouting {
	rs := make([]routing.ContentRouting, len(hosts))
	for i := 0; i < len(hosts); i++ {
		rs[i] = newMockRouting(ms, ks, hosts[i])
	}
	return rs
}

// tests
func TestPubsubPublishSubscribe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ms := mockrouting.NewServer()
	ks := newMockKeyStore()

	pubhost := newNetHost(ctx, t)
	pubmr := newMockRouting(ms, ks, pubhost)
	pub := NewPubsubPublisher(ctx, pubhost, ds.NewMapDatastore(), pubmr, floodsub.NewFloodSub(ctx, pubhost))
	privk := pubhost.Peerstore().PrivKey(pubhost.ID())
	pubpinfo := pstore.PeerInfo{ID: pubhost.ID(), Addrs: pubhost.Addrs()}

	name := "/ipns/" + pubhost.ID().Pretty()

	reshosts := newNetHosts(ctx, t, 5)
	resmrs := newMockRoutingForHosts(ms, ks, reshosts)
	res := make([]*PubsubResolver, len(reshosts))
	for i := 0; i < len(res); i++ {
		res[i] = NewPubsubResolver(ctx, reshosts[i], resmrs[i], ks, floodsub.NewFloodSub(ctx, reshosts[i]))
		if err := reshosts[i].Connect(ctx, pubpinfo); err != nil {
			t.Fatal(err)
		}
	}

	time.Sleep(time.Millisecond * 100)
	for i := 0; i < len(res); i++ {
		checkResolveNotFound(ctx, t, i, res[i], name)
		// delay to avoid connection storms
		time.Sleep(time.Millisecond * 100)
	}

	// let the bootstrap finish
	time.Sleep(time.Second * 1)

	val := path.Path("/ipfs/QmP1DfoUjiWH2ZBo1PBH6FupdBucbDepx3HpWmEY6JMUpY")
	err := pub.Publish(ctx, privk, val)
	if err != nil {
		t.Fatal(err)
	}

	// let the flood propagate
	time.Sleep(time.Second * 1)
	for i := 0; i < len(res); i++ {
		checkResolve(ctx, t, i, res[i], name, val)
	}

	val = path.Path("/ipfs/QmP1wMAqk6aZYRZirbaAwmrNeqFRgQrwBt3orUtvSa1UYD")
	err = pub.Publish(ctx, privk, val)
	if err != nil {
		t.Fatal(err)
	}

	// let the flood propagate
	time.Sleep(time.Second * 1)
	for i := 0; i < len(res); i++ {
		checkResolve(ctx, t, i, res[i], name, val)
	}

	// cancel subscriptions
	for i := 0; i < len(res); i++ {
		res[i].Cancel(name)
	}
	time.Sleep(time.Millisecond * 100)

	nval := path.Path("/ipfs/QmPgDWmTmuzvP7QE5zwo1TmjbJme9pmZHNujB2453jkCTr")
	err = pub.Publish(ctx, privk, nval)
	if err != nil {
		t.Fatal(err)
	}

	// check we still have the old value in the resolver
	time.Sleep(time.Second * 1)
	for i := 0; i < len(res); i++ {
		checkResolve(ctx, t, i, res[i], name, val)
	}
}

func checkResolveNotFound(ctx context.Context, t *testing.T, i int, resolver Resolver, name string) {
	_, err := resolver.Resolve(ctx, name)
	if err != ErrResolveFailed {
		t.Fatalf("[resolver %d] unexpected error: %s", i, err.Error())
	}
}

func checkResolve(ctx context.Context, t *testing.T, i int, resolver Resolver, name string, val path.Path) {
	xval, err := resolver.Resolve(ctx, name)
	if err != nil {
		t.Fatalf("[resolver %d] resolve failed: %s", i, err.Error())
	}
	if xval != val {
		t.Fatalf("[resolver %d] unexpected value: %s %s", i, val, xval)
	}
}
