package bitswap

import (
	"bytes"
	"testing"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/peer"
	mock "github.com/jbenet/go-ipfs/routing/mock"
	u "github.com/jbenet/go-ipfs/util"
)

func TestKeyNotFound(t *testing.T) {

	vrs := mock.VirtualRoutingServer()
	empty := vrs.Providers(u.Key("not there"))
	if len(empty) != 0 {
		t.Fatal("should be empty")
	}
}

func TestSetAndGet(t *testing.T) {
	pid := peer.ID([]byte("the peer id"))
	p := &peer.Peer{
		ID: pid,
	}
	k := u.Key("42")
	rs := mock.VirtualRoutingServer()
	err := rs.Announce(p, k)
	if err != nil {
		t.Fatal(err)
	}
	providers := rs.Providers(k)
	if len(providers) != 1 {
		t.Fatal("should be one")
	}
	for _, elem := range providers {
		if bytes.Equal(elem.ID, pid) {
			return
		}
	}
	t.Fatal("ID should have matched")
}

func TestClientFindProviders(t *testing.T) {
	peer := &peer.Peer{
		ID: []byte("42"),
	}
	rs := mock.VirtualRoutingServer()
	client := mock.NewMockRouter(peer, nil)
	client.SetRoutingServer(rs)
	k := u.Key("hello")
	err := client.Provide(context.Background(), k)
	if err != nil {
		t.Fatal(err)
	}
	max := 100

	providersFromHashTable := rs.Providers(k)

	isInHT := false
	for _, p := range providersFromHashTable {
		if bytes.Equal(p.ID, peer.ID) {
			isInHT = true
		}
	}
	if !isInHT {
		t.Fatal("Despite client providing key, peer wasn't in hash table as a provider")
	}
	providersFromClient := client.FindProvidersAsync(context.Background(), u.Key("hello"), max)
	isInClient := false
	for p := range providersFromClient {
		if bytes.Equal(p.ID, peer.ID) {
			isInClient = true
		}
	}
	if !isInClient {
		t.Fatal("Despite client providing key, client didn't receive peer when finding providers")
	}
}

func TestClientOverMax(t *testing.T) {
	rs := mock.VirtualRoutingServer()
	k := u.Key("hello")
	numProvidersForHelloKey := 100
	for i := 0; i < numProvidersForHelloKey; i++ {
		peer := &peer.Peer{
			ID: []byte(string(i)),
		}
		err := rs.Announce(peer, k)
		if err != nil {
			t.Fatal(err)
		}
	}
	providersFromHashTable := rs.Providers(k)
	if len(providersFromHashTable) != numProvidersForHelloKey {
		t.Log(1 == len(providersFromHashTable))
		t.Fatal("not all providers were returned")
	}

	max := 10
	client := mock.NewMockRouter(&peer.Peer{ID: []byte("TODO")}, nil)
	client.SetRoutingServer(rs)
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
	rs := mock.VirtualRoutingServer()
	k := u.Key("hello")

	t.Log("async'ly announce infinite stream of providers for key")
	i := 0
	go func() { // infinite stream
		for {
			peer := &peer.Peer{
				ID: []byte(string(i)),
			}
			err := rs.Announce(peer, k)
			if err != nil {
				t.Fatal(err)
			}
			i++
		}
	}()

	local := &peer.Peer{ID: []byte("peer id doesn't matter")}
	client := mock.NewMockRouter(local, nil)
	client.SetRoutingServer(rs)

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
