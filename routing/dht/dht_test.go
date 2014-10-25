package dht

import (
	"bytes"
	"testing"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	ci "github.com/jbenet/go-ipfs/crypto"
	inet "github.com/jbenet/go-ipfs/net"
	mux "github.com/jbenet/go-ipfs/net/mux"
	netservice "github.com/jbenet/go-ipfs/net/service"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"

	"fmt"
	"time"
)

func setupDHT(ctx context.Context, t *testing.T, p peer.Peer) *IpfsDHT {
	peerstore := peer.NewPeerstore()

	dhts := netservice.NewService(ctx, nil) // nil handler for now, need to patch it
	net, err := inet.NewIpfsNetwork(ctx, p, peerstore, &mux.ProtocolMap{
		mux.ProtocolID_Routing: dhts,
	})
	if err != nil {
		t.Fatal(err)
	}

	d := NewDHT(ctx, p, peerstore, net, dhts, ds.NewMapDatastore())
	dhts.SetHandler(d)
	return d
}

func setupDHTS(ctx context.Context, n int, t *testing.T) ([]ma.Multiaddr, []peer.Peer, []*IpfsDHT) {
	var addrs []ma.Multiaddr
	for i := 0; i < n; i++ {
		a, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", 5000+i))
		if err != nil {
			t.Fatal(err)
		}
		addrs = append(addrs, a)
	}

	var peers []peer.Peer
	for i := 0; i < n; i++ {
		p := makePeer(addrs[i])
		peers = append(peers, p)
	}

	dhts := make([]*IpfsDHT, n)
	for i := 0; i < n; i++ {
		dhts[i] = setupDHT(ctx, t, peers[i])
	}

	return addrs, peers, dhts
}

func makePeer(addr ma.Multiaddr) peer.Peer {
	sk, pk, err := ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		panic(err)
	}
	p, err := peer.WithKeyPair(sk, pk)
	if err != nil {
		panic(err)
	}
	p.AddAddress(addr)
	return p
}

func TestPing(t *testing.T) {
	// t.Skip("skipping test to debug another")
	ctx := context.Background()
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

	dhtA := setupDHT(ctx, t, peerA)
	dhtB := setupDHT(ctx, t, peerB)

	defer dhtA.Close()
	defer dhtB.Close()
	defer dhtA.dialer.(inet.Network).Close()
	defer dhtB.dialer.(inet.Network).Close()

	_, err = dhtA.Connect(ctx, peerB)
	if err != nil {
		t.Fatal(err)
	}

	//Test that we can ping the node
	ctxT, _ := context.WithTimeout(ctx, 100*time.Millisecond)
	err = dhtA.Ping(ctxT, peerB)
	if err != nil {
		t.Fatal(err)
	}

	ctxT, _ = context.WithTimeout(ctx, 100*time.Millisecond)
	err = dhtB.Ping(ctxT, peerA)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValueGetSet(t *testing.T) {
	// t.Skip("skipping test to debug another")

	ctx := context.Background()
	u.Debug = false
	addrA, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/11235")
	if err != nil {
		t.Fatal(err)
	}
	addrB, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/15679")
	if err != nil {
		t.Fatal(err)
	}

	peerA := makePeer(addrA)
	peerB := makePeer(addrB)

	dhtA := setupDHT(ctx, t, peerA)
	dhtB := setupDHT(ctx, t, peerB)

	defer dhtA.Close()
	defer dhtB.Close()
	defer dhtA.dialer.(inet.Network).Close()
	defer dhtB.dialer.(inet.Network).Close()

	_, err = dhtA.Connect(ctx, peerB)
	if err != nil {
		t.Fatal(err)
	}

	ctxT, _ := context.WithTimeout(ctx, time.Second)
	dhtA.PutValue(ctxT, "hello", []byte("world"))

	ctxT, _ = context.WithTimeout(ctx, time.Second*2)
	val, err := dhtA.GetValue(ctxT, "hello")
	if err != nil {
		t.Fatal(err)
	}

	if string(val) != "world" {
		t.Fatalf("Expected 'world' got '%s'", string(val))
	}

	ctxT, _ = context.WithTimeout(ctx, time.Second*2)
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
	ctx := context.Background()

	u.Debug = false

	_, peers, dhts := setupDHTS(ctx, 4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Close()
			defer dhts[i].dialer.(inet.Network).Close()
		}
	}()

	_, err := dhts[0].Connect(ctx, peers[1])
	if err != nil {
		t.Fatal(err)
	}

	_, err = dhts[1].Connect(ctx, peers[2])
	if err != nil {
		t.Fatal(err)
	}

	_, err = dhts[1].Connect(ctx, peers[3])
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

	err = dhts[3].Provide(ctx, u.Key("hello"))
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond * 60)

	ctxT, _ := context.WithTimeout(ctx, time.Second)
	provchan := dhts[0].FindProvidersAsync(ctxT, u.Key("hello"), 1)

	after := time.After(time.Second)
	select {
	case prov := <-provchan:
		if prov == nil {
			t.Fatal("Got back nil provider")
		}
	case <-after:
		t.Fatal("Did not get a provider back.")
	}
}

func TestProvidesAsync(t *testing.T) {
	// t.Skip("skipping test to debug another")

	ctx := context.Background()
	u.Debug = false

	_, peers, dhts := setupDHTS(ctx, 4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Close()
			defer dhts[i].dialer.(inet.Network).Close()
		}
	}()

	_, err := dhts[0].Connect(ctx, peers[1])
	if err != nil {
		t.Fatal(err)
	}

	_, err = dhts[1].Connect(ctx, peers[2])
	if err != nil {
		t.Fatal(err)
	}

	_, err = dhts[1].Connect(ctx, peers[3])
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

	err = dhts[3].Provide(ctx, u.Key("hello"))
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond * 60)

	ctxT, _ := context.WithTimeout(ctx, time.Millisecond*300)
	provs := dhts[0].FindProvidersAsync(ctxT, u.Key("hello"), 5)
	select {
	case p, ok := <-provs:
		if !ok {
			t.Fatal("Provider channel was closed...")
		}
		if p == nil {
			t.Fatal("Got back nil provider!")
		}
		if !p.ID().Equal(dhts[3].self.ID()) {
			t.Fatalf("got a provider, but not the right one. %s", p)
		}
	case <-ctxT.Done():
		t.Fatal("Didnt get back providers")
	}
}

func TestLayeredGet(t *testing.T) {
	// t.Skip("skipping test to debug another")

	ctx := context.Background()
	u.Debug = false
	_, peers, dhts := setupDHTS(ctx, 4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Close()
			defer dhts[i].dialer.(inet.Network).Close()
		}
	}()

	_, err := dhts[0].Connect(ctx, peers[1])
	if err != nil {
		t.Fatalf("Failed to connect: %s", err)
	}

	_, err = dhts[1].Connect(ctx, peers[2])
	if err != nil {
		t.Fatal(err)
	}

	_, err = dhts[1].Connect(ctx, peers[3])
	if err != nil {
		t.Fatal(err)
	}

	err = dhts[3].putLocal(u.Key("hello"), []byte("world"))
	if err != nil {
		t.Fatal(err)
	}

	err = dhts[3].Provide(ctx, u.Key("hello"))
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond * 60)

	ctxT, _ := context.WithTimeout(ctx, time.Second)
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

	ctx := context.Background()
	u.Debug = false

	_, peers, dhts := setupDHTS(ctx, 4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Close()
			dhts[i].dialer.(inet.Network).Close()
		}
	}()

	_, err := dhts[0].Connect(ctx, peers[1])
	if err != nil {
		t.Fatal(err)
	}

	_, err = dhts[1].Connect(ctx, peers[2])
	if err != nil {
		t.Fatal(err)
	}

	_, err = dhts[1].Connect(ctx, peers[3])
	if err != nil {
		t.Fatal(err)
	}

	ctxT, _ := context.WithTimeout(ctx, time.Second)
	p, err := dhts[0].FindPeer(ctxT, peers[2].ID())
	if err != nil {
		t.Fatal(err)
	}

	if p == nil {
		t.Fatal("Failed to find peer.")
	}

	if !p.ID().Equal(peers[2].ID()) {
		t.Fatal("Didnt find expected peer.")
	}
}

func TestConnectCollision(t *testing.T) {
	// t.Skip("skipping test to debug another")

	runTimes := 10

	for rtime := 0; rtime < runTimes; rtime++ {
		log.Notice("Running Time: ", rtime)

		ctx := context.Background()
		u.Debug = false
		addrA, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/11235")
		if err != nil {
			t.Fatal(err)
		}
		addrB, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/15679")
		if err != nil {
			t.Fatal(err)
		}

		peerA := makePeer(addrA)
		peerB := makePeer(addrB)

		dhtA := setupDHT(ctx, t, peerA)
		dhtB := setupDHT(ctx, t, peerB)

		done := make(chan struct{})
		go func() {
			_, err = dhtA.Connect(ctx, peerB)
			if err != nil {
				t.Fatal(err)
			}
			done <- struct{}{}
		}()
		go func() {
			_, err = dhtB.Connect(ctx, peerA)
			if err != nil {
				t.Fatal(err)
			}
			done <- struct{}{}
		}()

		timeout := time.After(time.Second)
		select {
		case <-done:
		case <-timeout:
			t.Fatal("Timeout received!")
		}
		select {
		case <-done:
		case <-timeout:
			t.Fatal("Timeout received!")
		}

		dhtA.Close()
		dhtB.Close()
		dhtA.dialer.(inet.Network).Close()
		dhtB.dialer.(inet.Network).Close()

		<-time.After(200 * time.Millisecond)
	}
}
