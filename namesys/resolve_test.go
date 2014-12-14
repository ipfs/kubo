package namesys

import (
	"testing"

	ci "github.com/jbenet/go-ipfs/crypto"
	mockrouting "github.com/jbenet/go-ipfs/routing/mock"
	u "github.com/jbenet/go-ipfs/util"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

func TestRoutingResolve(t *testing.T) {
	d := mockrouting.NewServer().Client(testutil.RandIdentityOrFatal(t))

	resolver := NewRoutingResolver(d)
	publisher := NewRoutingPublisher(d)

	privk, pubk, err := ci.GenerateKeyPair(ci.RSA, 512, u.NewTimeSeededRand())
	if err != nil {
		t.Fatal(err)
	}

	err = publisher.Publish(privk, "Hello")
	if err == nil {
		t.Fatal("should have errored out when publishing a non-multihash val")
	}

	h := u.Key(u.Hash([]byte("Hello"))).Pretty()
	err = publisher.Publish(privk, h)
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

	if res != h {
		t.Fatal("Got back incorrect value.")
	}
}
