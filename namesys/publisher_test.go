package namesys

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	testutil "gx/ipfs/QmNfQbgBfARAtrYsBguChX6VJ5nbjeoYy1KdC36aaYWqG8/go-testutil"
	ci "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	mockrouting "gx/ipfs/QmYKPBQpSSWwmgNTvVE3vQdPoeqxwudPQnXJ4hU383RsSA/go-ipfs-routing/mock"
	ipns "gx/ipfs/QmYZdD9dRfHtoYt4qAFgtKoiPBAoXntmM3ZcktZVvAgB4s/go-ipns"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	dshelp "gx/ipfs/QmafbjCxKZEU87RYYCHX2pFP9Q6uLSLAScLr2VUErNTH1f/go-ipfs-ds-help"
	peer "gx/ipfs/QmbNepETomvmXfz1X5pHNFD2QuPqnqi47dTd94QJWSorQ3/go-libp2p-peer"
	ds "gx/ipfs/QmbQshXLNcCPRUGZv4sBGxnZNAHREA6MKeomkwihNXPZWP/go-datastore"
	dssync "gx/ipfs/QmbQshXLNcCPRUGZv4sBGxnZNAHREA6MKeomkwihNXPZWP/go-datastore/sync"
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
