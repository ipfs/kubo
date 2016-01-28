package namesys

import (
	"errors"
	"testing"
	"time"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore"
	key "github.com/ipfs/go-ipfs/blocks/key"
	path "github.com/ipfs/go-ipfs/path"
	mockrouting "github.com/ipfs/go-ipfs/routing/mock"
	u "github.com/ipfs/go-ipfs/util"
	testutil "github.com/ipfs/go-ipfs/util/testutil"
	peer "gx/ipfs/QmZxtCsPRgCnCXwVPUjcBiFckkG5NMYM4Pthwe6X4C8uQq/go-libp2p/p2p/peer"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

func TestRoutingResolve(t *testing.T) {
	d := mockrouting.NewServer().Client(testutil.RandIdentityOrFatal(t))
	dstore := ds.NewMapDatastore()

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
	res, err := resolver.Resolve(context.Background(), key.Key(pkhash).Pretty())
	if err != nil {
		t.Fatal(err)
	}

	if res != h {
		t.Fatal("Got back incorrect value.")
	}
}

func TestPrexistingExpiredRecord(t *testing.T) {
	dstore := ds.NewMapDatastore()
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
	dstore := ds.NewMapDatastore()
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
