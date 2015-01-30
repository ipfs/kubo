package namesys

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"testing"

	mockrouting "github.com/jbenet/go-ipfs/routing/mock"
	u "github.com/jbenet/go-ipfs/util"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

func TestRoutingResolve(t *testing.T) {
	d := mockrouting.NewServer().Client(testutil.RandIdentityOrFatal(t))

	resolver := NewRoutingResolver(d)
	publisher := NewRoutingPublisher(d)

	privk, pubk, err := testutil.RandTestRSAKeyPair(512)
	if err != nil {
		t.Fatal(err)
	}

	err = publisher.Publish(context.Background(), privk, "Hello")
	if err == nil {
		t.Fatal("should have errored out when publishing a non-multihash val")
	}

	h := u.Key(u.Hash([]byte("Hello")))
	err = publisher.Publish(context.Background(), privk, h)
	if err != nil {
		t.Fatal(err)
	}

	pubkb, err := pubk.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	pkhash := u.Hash(pubkb)
	res, err := resolver.Resolve(context.Background(), u.Key(pkhash).Pretty())
	if err != nil {
		t.Fatal(err)
	}

	if res != h {
		t.Fatal("Got back incorrect value.")
	}
}
