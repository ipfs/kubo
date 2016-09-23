package supernode

import (
	"testing"

	dhtpb "gx/ipfs/QmYvLYkYiVEi5LBHP2uFqiUaHqH7zWnEuRqoNEuGLNG6JB/go-libp2p-kad-dht/pb"
	datastore "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
	key "gx/ipfs/Qmce4Y4zg3sYr7xKM5UueS67vhNni6EeWgCRnb7MbLJMew/go-key"
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
