package namesys

import (
	"context"
	"fmt"
	"testing"
	"time"

	opts "github.com/ipfs/go-ipfs/namesys/opts"
	path "github.com/ipfs/go-ipfs/path"

	u "gx/ipfs/QmNiJuT8Ja3hMVpBHXv3Q6dwmperaQ6JjLtpMQgMCD7xvx/go-ipfs-util"
	mockrouting "gx/ipfs/QmPuPdzoG4b5uyYSQCjLEHB8NM593m3BW19UHX2jZ6Wzfm/go-ipfs-routing/mock"
	record "gx/ipfs/QmTUyK82BVPA6LmSzEJpfEunk9uBaQzWtMsNP917tVj4sT/go-libp2p-record"
	recordpb "gx/ipfs/QmTUyK82BVPA6LmSzEJpfEunk9uBaQzWtMsNP917tVj4sT/go-libp2p-record/pb"
	routing "gx/ipfs/QmUHRKTeaoASDvDj7cTAXsmjAY7KQ13ErtzkQHZQq6uFUz/go-libp2p-routing"
	testutil "gx/ipfs/QmUJzxQQ2kzwQubsMqBTr1NGDpLfh7pGA2E1oaJULcKDPq/go-testutil"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	peer "gx/ipfs/QmcJukH2sAFjY3HdBKq35WDzWoL3UUu2gt9wdfqZTUyM74/go-libp2p-peer"
	pstore "gx/ipfs/QmdeiKhUy1TVGBaKxt7y1QmBDLBdisSrLJ1x58Eoj4PXUh/go-libp2p-peerstore"
	ci "gx/ipfs/Qme1knMqwt1hKZbc1BmQFmnm9f36nyQGwXxPGVpVJ9rMK5/go-libp2p-crypto"
	ds "gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"
	dssync "gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore/sync"
)

func testValidatorCase(t *testing.T, priv ci.PrivKey, kbook pstore.KeyBook, ns string, key string, val []byte, eol time.Time, exp error) {
	validChecker := NewIpnsRecordValidator(kbook)

	p := path.Path("/ipfs/QmfM2r8seH2GiRaC4esTjeraXEachRt8ZsSeGaWTPLyMoG")
	entry, err := CreateRoutingEntryData(priv, p, 1, eol)
	if err != nil {
		t.Fatal(err)
	}

	data := val
	if data == nil {
		data, err = proto.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
	}
	rec := &record.ValidationRecord{
		Namespace: ns,
		Key:       key,
		Value:     data,
	}

	err = validChecker.Func(rec)
	if err != exp {
		params := fmt.Sprintf("namespace: %s\nkey: %s\neol: %s\n", ns, key, eol)
		if exp == nil {
			t.Fatalf("Unexpected error %s for params %s", err, params)
		} else if err == nil {
			t.Fatalf("Expected error %s but there was no error for params %s", exp, params)
		} else {
			t.Fatalf("Expected error %s but got %s for params %s", exp, err, params)
		}
	}
}

func TestValidator(t *testing.T) {
	ts := time.Now()

	priv, id, _, _ := genKeys(t)
	priv2, id2, _, _ := genKeys(t)
	kbook := pstore.NewPeerstore()
	kbook.AddPubKey(id, priv.GetPublic())
	emptyKbook := pstore.NewPeerstore()

	testValidatorCase(t, priv, kbook, "ipns", string(id), nil, ts.Add(time.Hour), nil)
	testValidatorCase(t, priv, kbook, "ipns", string(id), nil, ts.Add(time.Hour*-1), ErrExpiredRecord)
	testValidatorCase(t, priv, kbook, "ipns", string(id), []byte("bad data"), ts.Add(time.Hour), ErrBadRecord)
	testValidatorCase(t, priv, kbook, "ipns", "bad key", nil, ts.Add(time.Hour), ErrKeyFormat)
	testValidatorCase(t, priv, emptyKbook, "ipns", string(id), nil, ts.Add(time.Hour), ErrPublicKeyNotFound)
	testValidatorCase(t, priv2, kbook, "ipns", string(id2), nil, ts.Add(time.Hour), ErrPublicKeyNotFound)
	testValidatorCase(t, priv2, kbook, "ipns", string(id), nil, ts.Add(time.Hour), ErrSignature)
	testValidatorCase(t, priv, kbook, "", string(id), nil, ts.Add(time.Hour), ErrInvalidPath)
	testValidatorCase(t, priv, kbook, "wrong", string(id), nil, ts.Add(time.Hour), ErrInvalidPath)
}

func TestResolverValidation(t *testing.T) {
	ctx := context.Background()
	rid := testutil.RandIdentityOrFatal(t)
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	peerstore := pstore.NewPeerstore()

	vstore := newMockValueStore(rid, dstore, peerstore)
	vstore.Validator["ipns"] = NewIpnsRecordValidator(peerstore)
	vstore.Validator["pk"] = &record.ValidChecker{
		Func: func(r *record.ValidationRecord) error {
			return nil
		},
		Sign: false,
	}
	resolver := NewRoutingResolver(vstore, 0)

	// Create entry with expiry in one hour
	priv, id, _, ipnsDHTPath := genKeys(t)
	ts := time.Now()
	p := path.Path("/ipfs/QmfM2r8seH2GiRaC4esTjeraXEachRt8ZsSeGaWTPLyMoG")
	entry, err := CreateRoutingEntryData(priv, p, 1, ts.Add(time.Hour))
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
	resp, err := resolver.resolveOnce(ctx, id.Pretty(), opts.DefaultResolveOpts())
	if err != nil {
		t.Fatal(err)
	}
	if resp != p {
		t.Fatalf("Mismatch between published path %s and resolved path %s", p, resp)
	}

	// Create expired entry
	expiredEntry, err := CreateRoutingEntryData(priv, p, 1, ts.Add(-1*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	// Publish entry
	err = PublishEntry(ctx, vstore, ipnsDHTPath, expiredEntry)
	if err != nil {
		t.Fatal(err)
	}

	// Record should fail validation because entry is expired
	_, err = resolver.resolveOnce(ctx, id.Pretty(), opts.DefaultResolveOpts())
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
	err = PublishEntry(ctx, vstore, ipnsDHTPath2, entry)
	if err != nil {
		t.Fatal(err)
	}

	// Record should fail validation because public key defined by
	// ipns path doesn't match record signature
	_, err = resolver.resolveOnce(ctx, id2.Pretty(), opts.DefaultResolveOpts())
	if err == nil {
		t.Fatal("ValidateIpnsRecord should have failed signature verification")
	}

	// Publish entry without making public key available in peer store
	priv3, id3, pubkDHTPath3, ipnsDHTPath3 := genKeys(t)
	entry3, err := CreateRoutingEntryData(priv3, p, 1, ts.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	err = PublishEntry(ctx, vstore, ipnsDHTPath3, entry3)
	if err != nil {
		t.Fatal(err)
	}

	// Record should fail validation because public key is not available
	// in peer store or on network
	_, err = resolver.resolveOnce(ctx, id3.Pretty(), opts.DefaultResolveOpts())
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
	_, err = resolver.resolveOnce(ctx, id3.Pretty(), opts.DefaultResolveOpts())
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
	pubkDHTPath, ipnsDHTPath := IpnsKeysForID(pid)

	return priv, pid, pubkDHTPath, ipnsDHTPath
}

type mockValueStore struct {
	r         routing.ValueStore
	kbook     pstore.KeyBook
	Validator record.Validator
}

func newMockValueStore(id testutil.Identity, dstore ds.Datastore, kbook pstore.KeyBook) *mockValueStore {
	serv := mockrouting.NewServer()
	r := serv.ClientWithDatastore(context.Background(), id, dstore)
	return &mockValueStore{r, kbook, make(record.Validator)}
}

func (m *mockValueStore) GetValue(ctx context.Context, k string) ([]byte, error) {
	data, err := m.r.GetValue(ctx, k)
	if err != nil {
		return data, err
	}

	rec := new(recordpb.Record)
	rec.Key = proto.String(k)
	rec.Value = data
	if err = m.Validator.VerifyRecord(rec); err != nil {
		return nil, err
	}

	return data, err
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

func (m *mockValueStore) GetValues(ctx context.Context, k string, count int) ([]routing.RecvdVal, error) {
	vals, err := m.r.GetValues(ctx, k, count)
	if err != nil {
		return nil, err
	}
	valid := make([]routing.RecvdVal, 0, len(vals))
	for _, v := range vals {
		rec := new(recordpb.Record)
		rec.Key = proto.String(k)
		rec.Value = v.Val
		if err = m.Validator.VerifyRecord(rec); err == nil {
			valid = append(valid, v)
		}
	}
	return valid, nil
}

func (m *mockValueStore) PutValue(ctx context.Context, k string, d []byte) error {
	return m.r.PutValue(ctx, k, d)
}
