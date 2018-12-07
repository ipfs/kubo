package namesys

import (
	"context"
	"testing"
	"time"

	path "gx/ipfs/QmZErC2Ay6WuGi96CPg316PwitdwgLo6RxZRqVjJjRj2MR/go-path"

	opts "github.com/ipfs/go-ipfs/namesys/opts"

	ci "gx/ipfs/QmNiJiXwWE3kRhZrC5ej3kSjWHm337pYfhjLGSCDNKJP2s/go-libp2p-crypto"
	u "gx/ipfs/QmNohiVssaPw3KVLZik59DBVGTSm2dGvYT9eoXt5DQ36Yz/go-ipfs-util"
	ipns "gx/ipfs/QmPrt2JqvtFcgMBmYBjtZ5jFzq6HoFXy8PTwLb2Dpm2cGf/go-ipns"
	testutil "gx/ipfs/QmPuhRE325DR8ChNcFtgd6F1eANCHy1oohXZPpYop4xsK6/go-testutil"
	routing "gx/ipfs/QmRASJXJUFygM5qU4YrH7k7jD6S4Hg8nJmgqJ4bYJvLatd/go-libp2p-routing"
	ropts "gx/ipfs/QmRASJXJUFygM5qU4YrH7k7jD6S4Hg8nJmgqJ4bYJvLatd/go-libp2p-routing/options"
	peer "gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	pstore "gx/ipfs/QmZ9zH2FnLcxv1xyzFeUpDUeo55xEhZQHgveZijcxr7TLj/go-libp2p-peerstore"
	pstoremem "gx/ipfs/QmZ9zH2FnLcxv1xyzFeUpDUeo55xEhZQHgveZijcxr7TLj/go-libp2p-peerstore/pstoremem"
	mockrouting "gx/ipfs/QmdmWkx54g7VfVyxeG8ic84uf4G6Eq1GohuyKA3XDuJ8oC/go-ipfs-routing/mock"
	offline "gx/ipfs/QmdmWkx54g7VfVyxeG8ic84uf4G6Eq1GohuyKA3XDuJ8oC/go-ipfs-routing/offline"
	ds "gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore"
	dssync "gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore/sync"
	record "gx/ipfs/QmfARXVCzpwFXQdepAJZuqyNDgV9doEsMnVCo1ssmuSe1U/go-libp2p-record"
)

func TestResolverValidation(t *testing.T) {
	ctx := context.Background()
	rid := testutil.RandIdentityOrFatal(t)
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	peerstore := pstoremem.NewPeerstore()

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
	resp, err := resolve(ctx, resolver, id.Pretty(), opts.DefaultResolveOpts())
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
	_, err = resolve(ctx, resolver, id.Pretty(), opts.DefaultResolveOpts())
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
	_, err = resolve(ctx, resolver, id2.Pretty(), opts.DefaultResolveOpts())
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
	_, err = resolve(ctx, resolver, id3.Pretty(), opts.DefaultResolveOpts())
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
	_, err = resolve(ctx, resolver, id3.Pretty(), opts.DefaultResolveOpts())
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

func (m *mockValueStore) SearchValue(ctx context.Context, k string, opts ...ropts.Option) (<-chan []byte, error) {
	return m.r.SearchValue(ctx, k, opts...)
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
