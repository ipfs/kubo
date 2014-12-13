package mockrouting

import (
	"bytes"
	"testing"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

func TestKeyNotFound(t *testing.T) {

	var peer = testutil.NewPeerWithID(peer.ID([]byte("the peer id")))
	var key = u.Key("mock key")
	var ctx = context.Background()

	rs := NewServer()
	providers := rs.Client(peer).FindProvidersAsync(ctx, key, 10)
	_, ok := <-providers
	if ok {
		t.Fatal("should be closed")
	}
}

func TestClientFindProviders(t *testing.T) {
	peer := testutil.NewPeerWithIDString("42")
	rs := NewServer()
	client := rs.Client(peer)

	k := u.Key("hello")
	err := client.Provide(context.Background(), k)
	if err != nil {
		t.Fatal(err)
	}
	max := 100

	providersFromHashTable, err := rs.Client(peer).FindProviders(context.Background(), k)
	if err != nil {
		t.Fatal(err)
	}

	isInHT := false
	for _, p := range providersFromHashTable {
		if bytes.Equal(p.ID(), peer.ID()) {
			isInHT = true
		}
	}
	if !isInHT {
		t.Fatal("Despite client providing key, peer wasn't in hash table as a provider")
	}
	providersFromClient := client.FindProvidersAsync(context.Background(), u.Key("hello"), max)
	isInClient := false
	for p := range providersFromClient {
		if bytes.Equal(p.ID(), peer.ID()) {
			isInClient = true
		}
	}
	if !isInClient {
		t.Fatal("Despite client providing key, client didn't receive peer when finding providers")
	}
}

func TestClientOverMax(t *testing.T) {
	rs := NewServer()
	k := u.Key("hello")
	numProvidersForHelloKey := 100
	for i := 0; i < numProvidersForHelloKey; i++ {
		peer := testutil.NewPeerWithIDString(string(i))
		err := rs.Client(peer).Provide(context.Background(), k)
		if err != nil {
			t.Fatal(err)
		}
	}

	max := 10
	peer := testutil.NewPeerWithIDString("TODO")
	client := rs.Client(peer)

	providersFromClient := client.FindProvidersAsync(context.Background(), k, max)
	i := 0
	for _ = range providersFromClient {
		i++
	}
	if i != max {
		t.Fatal("Too many providers returned")
	}
}

// TODO does dht ensure won't receive self as a provider? probably not.
func TestCanceledContext(t *testing.T) {
	rs := NewServer()
	k := u.Key("hello")

	t.Log("async'ly announce infinite stream of providers for key")
	i := 0
	go func() { // infinite stream
		for {
			peer := testutil.NewPeerWithIDString(string(i))
			err := rs.Client(peer).Provide(context.Background(), k)
			if err != nil {
				t.Fatal(err)
			}
			i++
		}
	}()

	local := testutil.NewPeerWithIDString("peer id doesn't matter")
	client := rs.Client(local)

	t.Log("warning: max is finite so this test is non-deterministic")
	t.Log("context cancellation could simply take lower priority")
	t.Log("and result in receiving the max number of results")
	max := 1000

	t.Log("cancel the context before consuming")
	ctx, cancelFunc := context.WithCancel(context.Background())
	cancelFunc()
	providers := client.FindProvidersAsync(ctx, k, max)

	numProvidersReturned := 0
	for _ = range providers {
		numProvidersReturned++
	}
	t.Log(numProvidersReturned)

	if numProvidersReturned == max {
		t.Fatal("Context cancel had no effect")
	}
}
