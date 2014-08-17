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
	addrA, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/2222")
	if err != nil {
		t.Fatal(err)
	}
	addrB, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/5678")
	if err != nil {
		t.Fatal(err)
	}

	peerA := new(peer.Peer)
	peerA.AddAddress(addrA)
	peerA.ID = peer.ID([]byte("peerA"))

	peerB := new(peer.Peer)
	peerB.AddAddress(addrB)
	peerB.ID = peer.ID([]byte("peerB"))

	neta := swarm.NewSwarm(peerA)
	err = neta.Listen()
	if err != nil {
		t.Fatal(err)
	}
	dhtA := NewDHT(peerA, neta)

	netb := swarm.NewSwarm(peerB)
	err = netb.Listen()
	if err != nil {
		t.Fatal(err)
	}
	dhtB := NewDHT(peerB, netb)

	dhtA.Start()
	dhtB.Start()

	_, err = dhtA.Connect(addrB)
	if err != nil {
		t.Fatal(err)
	}

	//Test that we can ping the node
	err = dhtA.Ping(peerB, time.Second*2)
	if err != nil {
		t.Fatal(err)
	}

	dhtA.Halt()
	dhtB.Halt()
}

func TestValueGetSet(t *testing.T) {
	u.Debug = false
	addrA, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/1235")
	if err != nil {
		t.Fatal(err)
	}
	addrB, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/5679")
	if err != nil {
		t.Fatal(err)
	}

	peerA := new(peer.Peer)
	peerA.AddAddress(addrA)
	peerA.ID = peer.ID([]byte("peerA"))

	peerB := new(peer.Peer)
	peerB.AddAddress(addrB)
	peerB.ID = peer.ID([]byte("peerB"))

	neta := swarm.NewSwarm(peerA)
	err = neta.Listen()
	if err != nil {
		t.Fatal(err)
	}
	dhtA := NewDHT(peerA, neta)

	netb := swarm.NewSwarm(peerB)
	err = netb.Listen()
	if err != nil {
		t.Fatal(err)
	}
	dhtB := NewDHT(peerB, netb)

	dhtA.Start()
	dhtB.Start()

	errsa := dhtA.network.GetChan().Errors
	errsb := dhtB.network.GetChan().Errors
	go func() {
		select {
		case err := <-errsa:
			t.Fatal(err)
		case err := <-errsb:
			t.Fatal(err)
		}
	}()

	_, err = dhtA.Connect(addrB)
	if err != nil {
		t.Fatal(err)
	}

	dhtA.PutValue("hello", []byte("world"))

	val, err := dhtA.GetValue("hello", time.Second*2)
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

	err = dhts[3].putLocal(u.Key("hello"), []byte("world"))
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

	err = dhts[3].putLocal(u.Key("hello"), []byte("world"))
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
