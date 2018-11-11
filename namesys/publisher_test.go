package namesys

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	ci "gx/ipfs/QmNiJiXwWE3kRhZrC5ej3kSjWHm337pYfhjLGSCDNKJP2s/go-libp2p-crypto"
	ipns "gx/ipfs/QmR9UpasSQR4Mqq1qiJAfnY4SVBxJn7r639CxiLjx8dYGm/go-ipns"
	ma "gx/ipfs/QmRKLtwMw131aK7ugC3G7ybpumMz78YrJe5dzneyindvG1/go-multiaddr"
	testutil "gx/ipfs/QmZXjR5X1p4KrQ967cTsy4MymMzUM8mZECF3PV8UcN4o3g/go-testutil"
	dshelp "gx/ipfs/QmauEMWPoSqggfpSDHMMXuDn12DTd7TaFBvn39eeurzKT2/go-ipfs-ds-help"
	peer "gx/ipfs/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"
	mockrouting "gx/ipfs/QmdxhyAwBrnmJFsYPK6tyHh4Yy3gK8gbULErX1dRnpUMqu/go-ipfs-routing/mock"
	ds "gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore"
	dssync "gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore/sync"
)

type identity struct {
	testutil.PeerNetParams
}

func (p *identity) ID() peer.ID {
	return p.PeerNetParams.ID
}

func (p *identity) Address() ma.Multiaddr {
	return p.Addr
}

func (p *identity) PrivateKey() ci.PrivKey {
	return p.PrivKey
}

func (p *identity) PublicKey() ci.PubKey {
	return p.PubKey
}

func testNamekeyPublisher(t *testing.T, keyType int, expectedErr error, expectedExistence bool) {
	// Context
	ctx := context.Background()

	// Private key
	privKey, pubKey, err := ci.GenerateKeyPairWithReader(keyType, 2048, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	// ID
	id, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		t.Fatal(err)
	}

	// Value
	value := []byte("ipfs/TESTING")

	// Seqnum
	seqnum := uint64(0)

	// Eol
	eol := time.Now().Add(24 * time.Hour)

	// Routing value store
	p := testutil.PeerNetParams{
		ID:      id,
		PrivKey: privKey,
		PubKey:  pubKey,
		Addr:    testutil.ZeroLocalTCPAddress,
	}

	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	serv := mockrouting.NewServer()
	r := serv.ClientWithDatastore(context.Background(), &identity{p}, dstore)

	entry, err := ipns.Create(privKey, value, seqnum, eol)
	if err != nil {
		t.Fatal(err)
	}

	err = PutRecordToRouting(ctx, r, pubKey, entry)
	if err != nil {
		t.Fatal(err)
	}

	// Check for namekey existence in value store
	namekey := PkKeyForID(id)
	_, err = r.GetValue(ctx, namekey)
	if err != expectedErr {
		t.Fatal(err)
	}

	// Also check datastore for completeness
	key := dshelp.NewKeyFromBinary([]byte(namekey))
	exists, err := dstore.Has(key)
	if err != nil {
		t.Fatal(err)
	}

	if exists != expectedExistence {
		t.Fatal("Unexpected key existence in datastore")
	}
}

func TestRSAPublisher(t *testing.T) {
	testNamekeyPublisher(t, ci.RSA, nil, true)
}

func TestEd22519Publisher(t *testing.T) {
	testNamekeyPublisher(t, ci.Ed25519, ds.ErrNotFound, false)
}
