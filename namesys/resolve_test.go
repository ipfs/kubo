package namesys

import (
	"testing"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	ci "github.com/jbenet/go-ipfs/crypto"
	"github.com/jbenet/go-ipfs/peer"
	mock "github.com/jbenet/go-ipfs/routing/mock"
	u "github.com/jbenet/go-ipfs/util"
)

func TestRoutingResolve(t *testing.T) {
	local := &peer.Peer{
		ID: []byte("testID"),
	}
	lds := ds.NewMapDatastore()
	d := mock.NewMockRouter(local, lds)

	resolver := NewRoutingResolver(d)
	publisher := NewRoutingPublisher(d)

	privk, pubk, err := ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		t.Fatal(err)
	}

	err = publisher.Publish(privk, "Hello")
	if err != nil {
		t.Fatal(err)
	}

	pubkb, err := pubk.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	pkhash := u.Hash(pubkb)
	res, err := resolver.Resolve(u.Key(pkhash).Pretty())
	if err != nil {
		t.Fatal(err)
	}

	if res != "Hello" {
		t.Fatal("Got back incorrect value.")
	}
}
