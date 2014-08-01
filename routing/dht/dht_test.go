package dht

import (
	"testing"
	peer "github.com/jbenet/go-ipfs/peer"
	ma "github.com/jbenet/go-multiaddr"
	u "github.com/jbenet/go-ipfs/util"

	"time"
)

func TestPing(t *testing.T) {
	u.Debug = false
	addr_a,err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/1234")
	if err != nil {
		t.Fatal(err)
	}
	addr_b,err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/5678")
	if err != nil {
		t.Fatal(err)
	}

	peer_a := new(peer.Peer)
	peer_a.AddAddress(addr_a)
	peer_a.ID = peer.ID([]byte("peer_a"))

	peer_b := new(peer.Peer)
	peer_b.AddAddress(addr_b)
	peer_b.ID = peer.ID([]byte("peer_b"))

	dht_a,err := NewDHT(peer_a)
	if err != nil {
		t.Fatal(err)
	}

	dht_b,err := NewDHT(peer_b)
	if err != nil {
		t.Fatal(err)
	}


	dht_a.Start()
	dht_b.Start()

	err = dht_a.Connect(addr_b)
	if err != nil {
		t.Fatal(err)
	}

	//Test that we can ping the node
	err = dht_a.Ping(peer_b, time.Second * 2)
	if err != nil {
		t.Fatal(err)
	}
}
