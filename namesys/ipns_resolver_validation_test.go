package namesys

import (
	"context"
	"testing"
	"time"

	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	mockrouting "github.com/ipfs/go-ipfs-routing/mock"
	offline "github.com/ipfs/go-ipfs-routing/offline"
	ipns "github.com/ipfs/go-ipns"
	ipns_pb "github.com/ipfs/go-ipns/pb"
	path "github.com/ipfs/go-path"
	opts "github.com/ipfs/interface-go-ipfs-core/options/namesys"
	ci "github.com/libp2p/go-libp2p-core/crypto"
	peer "github.com/libp2p/go-libp2p-core/peer"
	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	routing "github.com/libp2p/go-libp2p-core/routing"
	"github.com/libp2p/go-libp2p-core/test"
	pstoremem "github.com/libp2p/go-libp2p-peerstore/pstoremem"
	record "github.com/libp2p/go-libp2p-record"
	testutil "github.com/libp2p/go-libp2p-testing/net"
)

func TestResolverValidation(t *testing.T) {
	t.Run("RSA",
		func(t *testing.T) {
			testResolverValidation(t, ci.RSA)
		})
	t.Run("Ed25519",
		func(t *testing.T) {
			testResolverValidation(t, ci.Ed25519)
		})
	t.Run("ECDSA",
		func(t *testing.T) {
			testResolverValidation(t, ci.ECDSA)
		})
	t.Run("Secp256k1",
		func(t *testing.T) {
			testResolverValidation(t, ci.Secp256k1)
		})
}

func testResolverValidation(t *testing.T, keyType int) {
	ctx := context.Background()
	rid := testutil.RandIdentityOrFatal(t)
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	peerstore := pstoremem.NewPeerstore()

	vstore := newMockValueStore(rid, dstore, peerstore)
	resolver := NewIpnsResolver(vstore)

	nvVstore := offline.NewOfflineRouter(dstore, mockrouting.MockValidator{})

	// Create entry with expiry in one hour
	priv, id, _, ipnsDHTPath := genKeys(t, keyType)
	ts := time.Now()
	p := []byte("/ipfs/QmfM2r8seH2GiRaC4esTjeraXEachRt8ZsSeGaWTPLyMoG")
	entry, err := createIPNSRecordWithEmbeddedPublicKey(priv, p, 1, ts.Add(time.Hour))
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
	expiredEntry, err := createIPNSRecordWithEmbeddedPublicKey(priv, p, 1, ts.Add(-1*time.Hour))
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
	priv2, id2, _, ipnsDHTPath2 := genKeys(t, keyType)

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

	// Try embedding the incorrect private key inside the entry
	if err := ipns.EmbedPublicKey(priv2.GetPublic(), entry); err != nil {
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
}

func genKeys(t *testing.T, keyType int) (ci.PrivKey, peer.ID, string, string) {
	bits := 0
	if keyType == ci.RSA {
		bits = 2048
	}

	sk, pk, err := test.RandTestKeyPair(keyType, bits)
	if err != nil {
		t.Fatal(err)
	}
	id, err := peer.IDFromPublicKey(pk)
	if err != nil {
		t.Fatal(err)
	}
	return sk, id, PkKeyForID(id), ipns.RecordKey(id)
}

func createIPNSRecordWithEmbeddedPublicKey(sk ci.PrivKey, val []byte, seq uint64, eol time.Time) (*ipns_pb.IpnsEntry, error) {
	entry, err := ipns.Create(sk, val, seq, eol)
	if err != nil {
		return nil, err
	}
	if err := ipns.EmbedPublicKey(sk.GetPublic(), entry); err != nil {
		return nil, err
	}

	return entry, nil
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

func (m *mockValueStore) GetValue(ctx context.Context, k string, opts ...routing.Option) ([]byte, error) {
	return m.r.GetValue(ctx, k, opts...)
}

func (m *mockValueStore) SearchValue(ctx context.Context, k string, opts ...routing.Option) (<-chan []byte, error) {
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

func (m *mockValueStore) PutValue(ctx context.Context, k string, d []byte, opts ...routing.Option) error {
	return m.r.PutValue(ctx, k, d, opts...)
}
