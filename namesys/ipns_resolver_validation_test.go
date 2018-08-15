package namesys

import (
	"context"
	"testing"
	"time"

	opts "github.com/ipfs/go-ipfs/namesys/opts"
	path "gx/ipfs/QmWMcvZbNvk5codeqbm7L89C9kqSwka4KaHnDb8HRnxsSL/go-path"

	u "gx/ipfs/QmPdKqUcHGFdeSpvjVoaTRPPstGif9GBZb5Q56RVw9o69A/go-ipfs-util"
	ci "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	mockrouting "gx/ipfs/QmRr8DpNhQMzsoqAitUrw43D82pyPXZkyUqarhSAfkrdaQ/go-ipfs-routing/mock"
	offline "gx/ipfs/QmRr8DpNhQMzsoqAitUrw43D82pyPXZkyUqarhSAfkrdaQ/go-ipfs-routing/offline"
	routing "gx/ipfs/QmSD6bSPcXaaR7LpQHjytLWQD7DrCsb415CWfpbd9Szemb/go-libp2p-routing"
	ropts "gx/ipfs/QmSD6bSPcXaaR7LpQHjytLWQD7DrCsb415CWfpbd9Szemb/go-libp2p-routing/options"
	record "gx/ipfs/QmUTQSGgjs8CHm9yBcUHicpRs7C9abhyZiBwjzCUp1pNgX/go-libp2p-record"
	ds "gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore"
	dssync "gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore/sync"
	ipns "gx/ipfs/QmVHij7PuWUFeLcmRbD1ykDwB1WZMYP8yixo9bprUb3QHG/go-ipns"
	testutil "gx/ipfs/QmXG74iiKQnDstVQq9fPFQEB6JTNSWBbAWE1qsq6L4E5sR/go-testutil"
	pstore "gx/ipfs/QmYLXCWN2myozZpx8Wx4UjrRuQuhY3YtWoMi6SHaXii6aM/go-libp2p-peerstore"
	peer "gx/ipfs/QmcZSzKEM5yDfpZbeEEZaVmaZ1zXm6JWTbrQZSB8hCVPzk/go-libp2p-peer"
)

func TestResolverValidation(t *testing.T) {
	ctx := context.Background()
	rid := testutil.RandIdentityOrFatal(t)
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	peerstore := pstore.NewPeerstore()

	vstore := newMockValueStore(rid, dstore, peerstore)
	resolver := NewIpnsResolver(vstore)

	nvVstore := offline.NewOfflineRouter(dstore, mockrouting.MockValidator{})

	// Create entry with expiry in one hour
	priv, id, _, ipnsDHTPath := genKeys(t)
	ts := time.Now()
	p := []byte("/ipfs/QmfM2r8seH2GiRaC4esTjeraXEachRt8ZsSeGaWTPLyMoG")
	entry, err := ipns.Create(priv, p, 1, ts.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	// Make peer's public key available in peer store
	err = peerstore.AddPubKey(id, priv.GetPublic())
	if err != nil {
		t.Fatal(err)
	}

	// Publish entry
	err = PublishEntry(ctx, vstore, ipnsDHTPath, entry)
	if err != nil {
		t.Fatal(err)
	}

	// Resolve entry
	resp, _, err := resolver.resolveOnce(ctx, id.Pretty(), opts.DefaultResolveOpts())
	if err != nil {
		t.Fatal(err)
	}
	if resp != path.Path(p) {
		t.Fatalf("Mismatch between published path %s and resolved path %s", p, resp)
	}

	// Create expired entry
	expiredEntry, err := ipns.Create(priv, p, 1, ts.Add(-1*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	// Publish entry
	err = PublishEntry(ctx, nvVstore, ipnsDHTPath, expiredEntry)
	if err != nil {
		t.Fatal(err)
	}

	// Record should fail validation because entry is expired
	_, _, err = resolver.resolveOnce(ctx, id.Pretty(), opts.DefaultResolveOpts())
	if err == nil {
		t.Fatal("ValidateIpnsRecord should have returned error")
	}

	// Create IPNS record path with a different private key
	priv2, id2, _, ipnsDHTPath2 := genKeys(t)

	// Make peer's public key available in peer store
	err = peerstore.AddPubKey(id2, priv2.GetPublic())
	if err != nil {
		t.Fatal(err)
	}

	// Publish entry
	err = PublishEntry(ctx, nvVstore, ipnsDHTPath2, entry)
	if err != nil {
		t.Fatal(err)
	}

	// Record should fail validation because public key defined by
	// ipns path doesn't match record signature
	_, _, err = resolver.resolveOnce(ctx, id2.Pretty(), opts.DefaultResolveOpts())
	if err == nil {
		t.Fatal("ValidateIpnsRecord should have failed signature verification")
	}

	// Publish entry without making public key available in peer store
	priv3, id3, pubkDHTPath3, ipnsDHTPath3 := genKeys(t)
	entry3, err := ipns.Create(priv3, p, 1, ts.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	err = PublishEntry(ctx, nvVstore, ipnsDHTPath3, entry3)
	if err != nil {
		t.Fatal(err)
	}

	// Record should fail validation because public key is not available
	// in peer store or on network
	_, _, err = resolver.resolveOnce(ctx, id3.Pretty(), opts.DefaultResolveOpts())
	if err == nil {
		t.Fatal("ValidateIpnsRecord should have failed because public key was not found")
	}

	// Publish public key to the network
	err = PublishPublicKey(ctx, vstore, pubkDHTPath3, priv3.GetPublic())
	if err != nil {
		t.Fatal(err)
	}

	// Record should now pass validation because resolver will ensure
	// public key is available in the peer store by looking it up in
	// the DHT, which causes the DHT to fetch it and cache it in the
	// peer store
	_, _, err = resolver.resolveOnce(ctx, id3.Pretty(), opts.DefaultResolveOpts())
	if err != nil {
		t.Fatal(err)
	}
}

func genKeys(t *testing.T) (ci.PrivKey, peer.ID, string, string) {
	sr := u.NewTimeSeededRand()
	priv, _, err := ci.GenerateKeyPairWithReader(ci.RSA, 1024, sr)
	if err != nil {
		t.Fatal(err)
	}

	// Create entry with expiry in one hour
	pid, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}

	return priv, pid, PkKeyForID(pid), ipns.RecordKey(pid)
}

type mockValueStore struct {
	r     routing.ValueStore
	kbook pstore.KeyBook
}

func newMockValueStore(id testutil.Identity, dstore ds.Datastore, kbook pstore.KeyBook) *mockValueStore {
	return &mockValueStore{
		r: offline.NewOfflineRouter(dstore, record.NamespacedValidator{
			"ipns": ipns.Validator{KeyBook: kbook},
			"pk":   record.PublicKeyValidator{},
		}),
		kbook: kbook,
	}
}

func (m *mockValueStore) GetValue(ctx context.Context, k string, opts ...ropts.Option) ([]byte, error) {
	return m.r.GetValue(ctx, k, opts...)
}

func (m *mockValueStore) GetPublicKey(ctx context.Context, p peer.ID) (ci.PubKey, error) {
	pk := m.kbook.PubKey(p)
	if pk != nil {
		return pk, nil
	}

	pkkey := routing.KeyForPublicKey(p)
	val, err := m.GetValue(ctx, pkkey)
	if err != nil {
		return nil, err
	}

	pk, err = ci.UnmarshalPublicKey(val)
	if err != nil {
		return nil, err
	}

	return pk, m.kbook.AddPubKey(p, pk)
}

func (m *mockValueStore) PutValue(ctx context.Context, k string, d []byte, opts ...ropts.Option) error {
	return m.r.PutValue(ctx, k, d, opts...)
}
