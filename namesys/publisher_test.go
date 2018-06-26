package namesys

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	dshelp "gx/ipfs/QmNP2u7bofwUQptHQGPfabGWtTCbxhNLSZKqbf1uzsup9V/go-ipfs-ds-help"
	mockrouting "gx/ipfs/QmWLQyLU7yopJnwMvpHM5VSMG4xmbKgcq6P246mDy9xy5E/go-ipfs-routing/mock"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	testutil "gx/ipfs/QmcW4FGAt24fdK1jBgWQn3yP4R9ZLyWQqjozv9QK7epRhL/go-testutil"
	peer "gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"
	ipns "gx/ipfs/Qmdue1XShFNi3mpizGx9NR9hyNEj6U2wEW93yGhKqKCFGN/go-ipns"
	ci "gx/ipfs/Qme1knMqwt1hKZbc1BmQFmnm9f36nyQGwXxPGVpVJ9rMK5/go-libp2p-crypto"
	ds "gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"
	dssync "gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore/sync"
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
