package supernode

import (
	"testing"

	key "github.com/ipfs/go-ipfs/blocks/key"
	dhtpb "github.com/ipfs/go-ipfs/routing/dht/pb"
	datastore "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore"
)

func TestPutProviderDoesntResultInDuplicates(t *testing.T) {
	routingBackend := datastore.NewMapDatastore()
	k := key.Key("foo")
	put := []*dhtpb.Message_Peer{
		convPeer("bob", "127.0.0.1/tcp/4001"),
		convPeer("alice", "10.0.0.10/tcp/4001"),
	}
	if err := putRoutingProviders(routingBackend, k, put); err != nil {
		t.Fatal(err)
	}
	if err := putRoutingProviders(routingBackend, k, put); err != nil {
		t.Fatal(err)
	}

	got, err := getRoutingProviders(routingBackend, k)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatal("should be 2 values, but there are", len(got))
	}
}

func convPeer(name string, addrs ...string) *dhtpb.Message_Peer {
	var rawAddrs [][]byte
	for _, addr := range addrs {
		rawAddrs = append(rawAddrs, []byte(addr))
	}
	return &dhtpb.Message_Peer{Id: &name, Addrs: rawAddrs}
}
