package dht

import (
	"bytes"
	"testing"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	ci "github.com/jbenet/go-ipfs/crypto"
	spipe "github.com/jbenet/go-ipfs/crypto/spipe"
	inet "github.com/jbenet/go-ipfs/net"
	mux "github.com/jbenet/go-ipfs/net/mux"
	netservice "github.com/jbenet/go-ipfs/net/service"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"

	"fmt"
	"time"
)

func setupDHT(t *testing.T, p *peer.Peer) *IpfsDHT {
	ctx := context.Background()

	peerstore := peer.NewPeerstore()

	dhts := netservice.NewService(nil) // nil handler for now, need to patch it
	if err := dhts.Start(ctx); err != nil {
		t.Fatal(err)
	}

	net, err := inet.NewIpfsNetwork(ctx, p, peerstore, &mux.ProtocolMap{
		mux.ProtocolID_Routing: dhts,
	})
	if err != nil {
		t.Fatal(err)
	}

	d := NewDHT(p, peerstore, net, dhts, ds.NewMapDatastore())
	dhts.Handler = d
	return d
}

func setupDHTS(n int, t *testing.T) ([]ma.Multiaddr, []*peer.Peer, []*IpfsDHT) {
	var addrs []ma.Multiaddr
	for i := 0; i < n; i++ {
		a, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", 5000+i))
		if err != nil {
			t.Fatal(err)
		}
		addrs = append(addrs, a)
	}

	var peers []*peer.Peer
	for i := 0; i < n; i++ {
		p := makePeer(addrs[i])
		peers = append(peers, p)
	}

	dhts := make([]*IpfsDHT, n)
	for i := 0; i < n; i++ {
		dhts[i] = setupDHT(t, peers[i])
	}

	return addrs, peers, dhts
}

func makePeer(addr ma.Multiaddr) *peer.Peer {
	p := new(peer.Peer)
	p.AddAddress(addr)
	sk, pk, err := ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		panic(err)
	}
	p.PrivKey = sk
	p.PubKey = pk
	id, err := spipe.IDFromPubKey(pk)
	if err != nil {
		panic(err)
	}

	p.ID = id
	return p
}

func TestPing(t *testing.T) {
	// t.Skip("skipping test to debug another")

	u.Debug = false
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

	dhtA := setupDHT(t, peerA)
	dhtB := setupDHT(t, peerB)

	defer dhtA.Halt()
	defer dhtB.Halt()
	defer dhtA.network.Close()
	defer dhtB.network.Close()

	_, err = dhtA.Connect(context.Background(), peerB)
	if err != nil {
		t.Fatal(err)
	}

	//Test that we can ping the node
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Millisecond)
	err = dhtA.Ping(ctx, peerB)
	if err != nil {
		t.Fatal(err)
	}

	ctx, _ = context.WithTimeout(context.Background(), 5*time.Millisecond)
	err = dhtB.Ping(ctx, peerA)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValueGetSet(t *testing.T) {
	// t.Skip("skipping test to debug another")

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

	dhtA := setupDHT(t, peerA)
	dhtB := setupDHT(t, peerB)

	defer dhtA.Halt()
	defer dhtB.Halt()
	defer dhtA.network.Close()
	defer dhtB.network.Close()

	_, err = dhtA.Connect(context.Background(), peerB)
	if err != nil {
		t.Fatal(err)
	}

	ctxT, _ := context.WithTimeout(context.Background(), time.Second)
	dhtA.PutValue(ctxT, "hello", []byte("world"))

	ctxT, _ = context.WithTimeout(context.Background(), time.Second*2)
	val, err := dhtA.GetValue(ctxT, "hello")
	if err != nil {
		t.Fatal(err)
	}

	if string(val) != "world" {
		t.Fatalf("Expected 'world' got '%s'", string(val))
	}

	ctxT, _ = context.WithTimeout(context.Background(), time.Second*2)
	val, err = dhtB.GetValue(ctxT, "hello")
	if err != nil {
		t.Fatal(err)
	}

	if string(val) != "world" {
		t.Fatalf("Expected 'world' got '%s'", string(val))
	}
}

func TestProvides(t *testing.T) {
	// t.Skip("skipping test to debug another")

	u.Debug = false

	_, peers, dhts := setupDHTS(4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Halt()
			defer dhts[i].network.Close()
		}
	}()

	_, err := dhts[0].Connect(context.Background(), peers[1])
	if err != nil {
		t.Fatal(err)
	}

	_, err = dhts[1].Connect(context.Background(), peers[2])
	if err != nil {
		t.Fatal(err)
	}

	_, err = dhts[1].Connect(context.Background(), peers[3])
	if err != nil {
		t.Fatal(err)
	}

	err = dhts[3].putLocal(u.Key("hello"), []byte("world"))
	if err != nil {
		t.Fatal(err)
	}

	bits, err := dhts[3].getLocal(u.Key("hello"))
	if err != nil && bytes.Equal(bits, []byte("world")) {
		t.Fatal(err)
	}

	err = dhts[3].Provide(context.Background(), u.Key("hello"))
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond * 60)

	ctxT, _ := context.WithTimeout(context.Background(), time.Second)
	provs, err := dhts[0].FindProviders(ctxT, u.Key("hello"))
	if err != nil {
		t.Fatal(err)
	}

	if len(provs) != 1 {
		t.Fatal("Didnt get back providers")
	}
}

func TestProvidesAsync(t *testing.T) {
	// t.Skip("skipping test to debug another")

	u.Debug = false

	_, peers, dhts := setupDHTS(4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Halt()
			defer dhts[i].network.Close()
		}
	}()

	_, err := dhts[0].Connect(context.Background(), peers[1])
	if err != nil {
		t.Fatal(err)
	}

	_, err = dhts[1].Connect(context.Background(), peers[2])
	if err != nil {
		t.Fatal(err)
	}

	_, err = dhts[1].Connect(context.Background(), peers[3])
	if err != nil {
		t.Fatal(err)
	}

	err = dhts[3].putLocal(u.Key("hello"), []byte("world"))
	if err != nil {
		t.Fatal(err)
	}

	bits, err := dhts[3].getLocal(u.Key("hello"))
	if err != nil && bytes.Equal(bits, []byte("world")) {
		t.Fatal(err)
	}

	err = dhts[3].Provide(context.Background(), u.Key("hello"))
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond * 60)

	ctx, _ := context.WithTimeout(context.TODO(), time.Millisecond*300)
	provs := dhts[0].FindProvidersAsync(ctx, u.Key("hello"), 5)
	select {
	case p := <-provs:
		if !p.ID.Equal(dhts[3].self.ID) {
			t.Fatalf("got a provider, but not the right one. %s", p)
		}
	case <-ctx.Done():
		t.Fatal("Didnt get back providers")
	}
}

func TestLayeredGet(t *testing.T) {
	// t.Skip("skipping test to debug another")

	u.Debug = false
	_, peers, dhts := setupDHTS(4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Halt()
			defer dhts[i].network.Close()
		}
	}()

	_, err := dhts[0].Connect(context.Background(), peers[1])
	if err != nil {
		t.Fatalf("Failed to connect: %s", err)
	}

	_, err = dhts[1].Connect(context.Background(), peers[2])
	if err != nil {
		t.Fatal(err)
	}

	_, err = dhts[1].Connect(context.Background(), peers[3])
	if err != nil {
		t.Fatal(err)
	}

	err = dhts[3].putLocal(u.Key("hello"), []byte("world"))
	if err != nil {
		t.Fatal(err)
	}

	err = dhts[3].Provide(context.Background(), u.Key("hello"))
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond * 60)

	ctxT, _ := context.WithTimeout(context.Background(), time.Second)
	val, err := dhts[0].GetValue(ctxT, u.Key("hello"))
	if err != nil {
		t.Fatal(err)
	}

	if string(val) != "world" {
		t.Fatal("Got incorrect value.")
	}

}

func TestFindPeer(t *testing.T) {
	// t.Skip("skipping test to debug another")

	u.Debug = false

	_, peers, dhts := setupDHTS(4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Halt()
			dhts[i].network.Close()
		}
	}()

	_, err := dhts[0].Connect(context.Background(), peers[1])
	if err != nil {
		t.Fatal(err)
	}

	_, err = dhts[1].Connect(context.Background(), peers[2])
	if err != nil {
		t.Fatal(err)
	}

	_, err = dhts[1].Connect(context.Background(), peers[3])
	if err != nil {
		t.Fatal(err)
	}

	ctxT, _ := context.WithTimeout(context.Background(), time.Second)
	p, err := dhts[0].FindPeer(ctxT, peers[2].ID)
	if err != nil {
		t.Fatal(err)
	}

	if p == nil {
		t.Fatal("Failed to find peer.")
	}

	if !p.ID.Equal(peers[2].ID) {
		t.Fatal("Didnt find expected peer.")
	}
}
