package namesys

import (
	"errors"
	"testing"
	"time"

	key "github.com/ipfs/go-ipfs/blocks/key"
	path "github.com/ipfs/go-ipfs/path"
	mockrouting "github.com/ipfs/go-ipfs/routing/mock"
	testutil "github.com/ipfs/go-ipfs/thirdparty/testutil"
	peer "gx/ipfs/QmQGwpJy9P4yXZySmqkZEXCmbBpJUb8xntCv8Ca4taZwDC/go-libp2p-peer"
	ds "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore"
	dssync "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore/sync"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

func TestRoutingResolve(t *testing.T) {
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	serv := mockrouting.NewServer()
	id := testutil.RandIdentityOrFatal(t)
	d := serv.ClientWithDatastore(context.Background(), id, dstore)

	resolver := NewRoutingResolver(d, 0)
	publisher := NewRoutingPublisher(d, dstore)

	privk, pubk, err := testutil.RandTestKeyPair(512)
	if err != nil {
		t.Fatal(err)
	}

	h := path.FromString("/ipfs/QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN")
	err = publisher.Publish(context.Background(), privk, h)
	if err != nil {
		t.Fatal(err)
	}

	pubkb, err := pubk.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	pkhash := u.Hash(pubkb)
	res, err := resolver.Resolve(context.Background(), key.Key(pkhash).B58String())
	if err != nil {
		t.Fatal(err)
	}

	if res != h {
		t.Fatal("Got back incorrect value.")
	}
}

func TestPrexistingExpiredRecord(t *testing.T) {
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	d := mockrouting.NewServer().ClientWithDatastore(context.Background(), testutil.RandIdentityOrFatal(t), dstore)

	resolver := NewRoutingResolver(d, 0)
	publisher := NewRoutingPublisher(d, dstore)

	privk, pubk, err := testutil.RandTestKeyPair(512)
	if err != nil {
		t.Fatal(err)
	}

	id, err := peer.IDFromPublicKey(pubk)
	if err != nil {
		t.Fatal(err)
	}

	// Make an expired record and put it in the datastore
	h := path.FromString("/ipfs/QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN")
	eol := time.Now().Add(time.Hour * -1)
	err = PutRecordToRouting(context.Background(), privk, h, 0, eol, d, id)
	if err != nil {
		t.Fatal(err)
	}

	// Now, with an old record in the system already, try and publish a new one
	err = publisher.Publish(context.Background(), privk, h)
	if err != nil {
		t.Fatal(err)
	}

	err = verifyCanResolve(resolver, id.Pretty(), h)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPrexistingRecord(t *testing.T) {
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	d := mockrouting.NewServer().ClientWithDatastore(context.Background(), testutil.RandIdentityOrFatal(t), dstore)

	resolver := NewRoutingResolver(d, 0)
	publisher := NewRoutingPublisher(d, dstore)

	privk, pubk, err := testutil.RandTestKeyPair(512)
	if err != nil {
		t.Fatal(err)
	}

	id, err := peer.IDFromPublicKey(pubk)
	if err != nil {
		t.Fatal(err)
	}

	// Make a good record and put it in the datastore
	h := path.FromString("/ipfs/QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN")
	eol := time.Now().Add(time.Hour)
	err = PutRecordToRouting(context.Background(), privk, h, 0, eol, d, id)
	if err != nil {
		t.Fatal(err)
	}

	// Now, with an old record in the system already, try and publish a new one
	err = publisher.Publish(context.Background(), privk, h)
	if err != nil {
		t.Fatal(err)
	}

	err = verifyCanResolve(resolver, id.Pretty(), h)
	if err != nil {
		t.Fatal(err)
	}
}

func verifyCanResolve(r Resolver, name string, exp path.Path) error {
	res, err := r.Resolve(context.Background(), name)
	if err != nil {
		return err
	}

	if res != exp {
		return errors.New("got back wrong record!")
	}

	return nil
}
