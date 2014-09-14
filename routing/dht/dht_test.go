package dht

import (
	"testing"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	ci "github.com/jbenet/go-ipfs/crypto"
	identify "github.com/jbenet/go-ipfs/identify"
	swarm "github.com/jbenet/go-ipfs/net/swarm"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"

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
		sk, pk, err := ci.GenerateKeyPair(ci.RSA, 512)
		if err != nil {
			panic(err)
		}
		p.PubKey = pk
		p.PrivKey = sk
		id, err := identify.IDFromPubKey(pk)
		if err != nil {
			panic(err)
		}
		p.ID = id
		peers = append(peers, p)
	}

	var dhts []*IpfsDHT
	for i := 0; i < 4; i++ {
		net := swarm.NewSwarm(peers[i])
		err := net.Listen()
		if err != nil {
			t.Fatal(err)
		}
		d := NewDHT(peers[i], net, ds.NewMapDatastore())
		dhts = append(dhts, d)
		d.Start()
	}

	return addrs, peers, dhts
}

func makePeer(addr *ma.Multiaddr) *peer.Peer {
	p := new(peer.Peer)
	p.AddAddress(addr)
	sk, pk, err := ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		panic(err)
	}
	p.PrivKey = sk
	p.PubKey = pk
	id, err := identify.IDFromPubKey(pk)
	if err != nil {
		panic(err)
	}

	p.ID = id
	return p
}

func TestPing(t *testing.T) {
	u.Debug = true
	addrA, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/2222")
	if err != nil {
		t.Fatal(err)
	}
	addrB, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/5678")
	if err != nil {
		t.Fatal(err)
	}

	peerA := makePeer(addrA)
	peerB := makePeer(addrB)

	neta := swarm.NewSwarm(peerA)
	err = neta.Listen()
	if err != nil {
		t.Fatal(err)
	}
	dhtA := NewDHT(peerA, neta, ds.NewMapDatastore())

	netb := swarm.NewSwarm(peerB)
	err = netb.Listen()
	if err != nil {
		t.Fatal(err)
	}
	dhtB := NewDHT(peerB, netb, ds.NewMapDatastore())

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

	peerA := makePeer(addrA)
	peerB := makePeer(addrB)

	neta := swarm.NewSwarm(peerA)
	err = neta.Listen()
	if err != nil {
		t.Fatal(err)
	}
	dhtA := NewDHT(peerA, neta, ds.NewMapDatastore())

	netb := swarm.NewSwarm(peerB)
	err = netb.Listen()
	if err != nil {
		t.Fatal(err)
	}
	dhtB := NewDHT(peerB, netb, ds.NewMapDatastore())

	dhtA.Start()
	dhtB.Start()

	errsa := dhtA.network.GetErrChan()
	errsb := dhtB.network.GetErrChan()
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
