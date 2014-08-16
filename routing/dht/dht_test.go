package dht

import (
	"testing"

	peer "github.com/jbenet/go-ipfs/peer"
	swarm "github.com/jbenet/go-ipfs/swarm"
	u "github.com/jbenet/go-ipfs/util"
	ma "github.com/jbenet/go-multiaddr"

	"fmt"
	"time"
)

func setupDHTS(n int, t *testing.T) ([]*ma.Multiaddr, []*peer.Peer, []*IpfsDHT) {
	var addrs []*ma.Multiaddr
	for i := 0; i < 4; i++ {
		a, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", 5000+i))
		if err != nil {
			t.Fatal(err)
		}
		addrs = append(addrs, a)
	}

	var peers []*peer.Peer
	for i := 0; i < 4; i++ {
		p := new(peer.Peer)
		p.AddAddress(addrs[i])
		p.ID = peer.ID([]byte(fmt.Sprintf("peer_%d", i)))
		peers = append(peers, p)
	}

	var dhts []*IpfsDHT
	for i := 0; i < 4; i++ {
		net := swarm.NewSwarm(peers[i])
		err := net.Listen()
		if err != nil {
			t.Fatal(err)
		}
		d := NewDHT(peers[i], net)
		dhts = append(dhts, d)
		d.Start()
	}

	return addrs, peers, dhts
}

func TestPing(t *testing.T) {
	u.Debug = false
	addr_a, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/2222")
	if err != nil {
		t.Fatal(err)
	}
	addr_b, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/5678")
	if err != nil {
		t.Fatal(err)
	}

	peer_a := new(peer.Peer)
	peer_a.AddAddress(addr_a)
	peer_a.ID = peer.ID([]byte("peer_a"))

	peer_b := new(peer.Peer)
	peer_b.AddAddress(addr_b)
	peer_b.ID = peer.ID([]byte("peer_b"))

	neta := swarm.NewSwarm(peer_a)
	err = neta.Listen()
	if err != nil {
		t.Fatal(err)
	}
	dht_a := NewDHT(peer_a, neta)

	netb := swarm.NewSwarm(peer_b)
	err = netb.Listen()
	if err != nil {
		t.Fatal(err)
	}
	dht_b := NewDHT(peer_b, netb)

	dht_a.Start()
	dht_b.Start()

	_, err = dht_a.Connect(addr_b)
	if err != nil {
		t.Fatal(err)
	}

	//Test that we can ping the node
	err = dht_a.Ping(peer_b, time.Second*2)
	if err != nil {
		t.Fatal(err)
	}

	dht_a.Halt()
	dht_b.Halt()
}

func TestValueGetSet(t *testing.T) {
	u.Debug = false
	addr_a, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/1235")
	if err != nil {
		t.Fatal(err)
	}
	addr_b, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/5679")
	if err != nil {
		t.Fatal(err)
	}

	peer_a := new(peer.Peer)
	peer_a.AddAddress(addr_a)
	peer_a.ID = peer.ID([]byte("peer_a"))

	peer_b := new(peer.Peer)
	peer_b.AddAddress(addr_b)
	peer_b.ID = peer.ID([]byte("peer_b"))

	neta := swarm.NewSwarm(peer_a)
	err = neta.Listen()
	if err != nil {
		t.Fatal(err)
	}
	dht_a := NewDHT(peer_a, neta)

	netb := swarm.NewSwarm(peer_b)
	err = netb.Listen()
	if err != nil {
		t.Fatal(err)
	}
	dht_b := NewDHT(peer_b, netb)

	dht_a.Start()
	dht_b.Start()

	errsa := dht_a.network.GetChan().Errors
	errsb := dht_b.network.GetChan().Errors
	go func() {
		select {
		case err := <-errsa:
			t.Fatal(err)
		case err := <-errsb:
			t.Fatal(err)
		}
	}()

	_, err = dht_a.Connect(addr_b)
	if err != nil {
		t.Fatal(err)
	}

	dht_a.PutValue("hello", []byte("world"))

	val, err := dht_a.GetValue("hello", time.Second*2)
	if err != nil {
		t.Fatal(err)
	}

	if string(val) != "world" {
		t.Fatalf("Expected 'world' got '%s'", string(val))
	}
}

func TestProvides(t *testing.T) {
	u.Debug = false

	addrs, _, dhts := setupDHTS(4, t)

	_, err := dhts[0].Connect(addrs[1])
	if err != nil {
		t.Fatal(err)
	}

	_, err = dhts[1].Connect(addrs[2])
	if err != nil {
		t.Fatal(err)
	}

	_, err = dhts[1].Connect(addrs[3])
	if err != nil {
		t.Fatal(err)
	}

	err = dhts[3].PutLocal(u.Key("hello"), []byte("world"))
	if err != nil {
		t.Fatal(err)
	}

	err = dhts[3].Provide(u.Key("hello"))
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond * 60)

	provs, err := dhts[0].FindProviders(u.Key("hello"), time.Second)
	if err != nil {
		t.Fatal(err)
	}

	if len(provs) != 1 {
		t.Fatal("Didnt get back providers")
	}

	for i := 0; i < 4; i++ {
		dhts[i].Halt()
	}
}

func TestLayeredGet(t *testing.T) {
	u.Debug = false
	addrs, _, dhts := setupDHTS(4, t)

	_, err := dhts[0].Connect(addrs[1])
	if err != nil {
		t.Fatalf("Failed to connect: %s", err)
	}

	_, err = dhts[1].Connect(addrs[2])
	if err != nil {
		t.Fatal(err)
	}

	_, err = dhts[1].Connect(addrs[3])
	if err != nil {
		t.Fatal(err)
	}

	err = dhts[3].PutLocal(u.Key("hello"), []byte("world"))
	if err != nil {
		t.Fatal(err)
	}

	err = dhts[3].Provide(u.Key("hello"))
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond * 60)

	val, err := dhts[0].GetValue(u.Key("hello"), time.Second)
	if err != nil {
		t.Fatal(err)
	}

	if string(val) != "world" {
		t.Fatal("Got incorrect value.")
	}

	for i := 0; i < 4; i++ {
		dhts[i].Halt()
	}
}

func TestFindPeer(t *testing.T) {
	u.Debug = false

	addrs, peers, dhts := setupDHTS(4, t)

	_, err := dhts[0].Connect(addrs[1])
	if err != nil {
		t.Fatal(err)
	}

	_, err = dhts[1].Connect(addrs[2])
	if err != nil {
		t.Fatal(err)
	}

	_, err = dhts[1].Connect(addrs[3])
	if err != nil {
		t.Fatal(err)
	}

	p, err := dhts[0].FindPeer(peers[2].ID, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	if p == nil {
		t.Fatal("Failed to find peer.")
	}

	if !p.ID.Equal(peers[2].ID) {
		t.Fatal("Didnt find expected peer.")
	}

	for i := 0; i < 4; i++ {
		dhts[i].Halt()
	}
}
