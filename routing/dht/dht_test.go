package dht

import (
	"bytes"
	"sort"
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
	testutil "github.com/jbenet/go-ipfs/util/testutil"

	"fmt"
	"time"
)

func setupDHT(ctx context.Context, t *testing.T, p peer.Peer) *IpfsDHT {
	peerstore := peer.NewPeerstore()

	dhts := netservice.NewService(ctx, nil) // nil handler for now, need to patch it
	net, err := inet.NewIpfsNetwork(ctx, p.Addresses(), p, peerstore, &mux.ProtocolMap{
		mux.ProtocolID_Routing: dhts,
	})
	if err != nil {
		t.Fatal(err)
	}

	d := NewDHT(ctx, p, peerstore, net, dhts, ds.NewMapDatastore())
	dhts.SetHandler(d)
	d.Validators["v"] = func(u.Key, []byte) error {
		return nil
	}
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

func makePeerString(t *testing.T, addr string) peer.Peer {
	maddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		t.Fatal(err)
	}
	return makePeer(maddr)
}

func makePeer(addr ma.Multiaddr) peer.Peer {
	sk, pk, err := ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		panic(err)
	}
	p, err := testutil.NewPeerWithKeyPair(sk, pk)
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

	err = dhtA.Connect(ctx, peerB)
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

	vf := func(u.Key, []byte) error {
		return nil
	}
	dhtA.Validators["v"] = vf
	dhtB.Validators["v"] = vf

	defer dhtA.Close()
	defer dhtB.Close()
	defer dhtA.dialer.(inet.Network).Close()
	defer dhtB.dialer.(inet.Network).Close()

	err = dhtA.Connect(ctx, peerB)
	if err != nil {
		t.Fatal(err)
	}

	ctxT, _ := context.WithTimeout(ctx, time.Second)
	dhtA.PutValue(ctxT, "/v/hello", []byte("world"))

	ctxT, _ = context.WithTimeout(ctx, time.Second*2)
	val, err := dhtA.GetValue(ctxT, "/v/hello")
	if err != nil {
		t.Fatal(err)
	}

	if string(val) != "world" {
		t.Fatalf("Expected 'world' got '%s'", string(val))
	}

	ctxT, _ = context.WithTimeout(ctx, time.Second*2)
	val, err = dhtB.GetValue(ctxT, "/v/hello")
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

	err := dhts[0].Connect(ctx, peers[1])
	if err != nil {
		t.Fatal(err)
	}

	err = dhts[1].Connect(ctx, peers[2])
	if err != nil {
		t.Fatal(err)
	}

	err = dhts[1].Connect(ctx, peers[3])
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
	if testing.Short() {
		t.SkipNow()
	}

	ctx := context.Background()
	u.Debug = false

	_, peers, dhts := setupDHTS(ctx, 4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Close()
			defer dhts[i].dialer.(inet.Network).Close()
		}
	}()

	err := dhts[0].Connect(ctx, peers[1])
	if err != nil {
		t.Fatal(err)
	}

	err = dhts[1].Connect(ctx, peers[2])
	if err != nil {
		t.Fatal(err)
	}

	err = dhts[1].Connect(ctx, peers[3])
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
	if testing.Short() {
		t.SkipNow()
	}

	ctx := context.Background()
	u.Debug = false
	_, peers, dhts := setupDHTS(ctx, 4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Close()
			defer dhts[i].dialer.(inet.Network).Close()
		}
	}()

	err := dhts[0].Connect(ctx, peers[1])
	if err != nil {
		t.Fatalf("Failed to connect: %s", err)
	}

	err = dhts[1].Connect(ctx, peers[2])
	if err != nil {
		t.Fatal(err)
	}

	err = dhts[1].Connect(ctx, peers[3])
	if err != nil {
		t.Fatal(err)
	}

	err = dhts[3].putLocal(u.Key("/v/hello"), []byte("world"))
	if err != nil {
		t.Fatal(err)
	}

	err = dhts[3].Provide(ctx, u.Key("/v/hello"))
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond * 60)

	ctxT, _ := context.WithTimeout(ctx, time.Second)
	val, err := dhts[0].GetValue(ctxT, u.Key("/v/hello"))
	if err != nil {
		t.Fatal(err)
	}

	if string(val) != "world" {
		t.Fatal("Got incorrect value.")
	}

}

func TestFindPeer(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	ctx := context.Background()
	u.Debug = false

	_, peers, dhts := setupDHTS(ctx, 4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Close()
			dhts[i].dialer.(inet.Network).Close()
		}
	}()

	err := dhts[0].Connect(ctx, peers[1])
	if err != nil {
		t.Fatal(err)
	}

	err = dhts[1].Connect(ctx, peers[2])
	if err != nil {
		t.Fatal(err)
	}

	err = dhts[1].Connect(ctx, peers[3])
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

func TestFindPeersConnectedToPeer(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	ctx := context.Background()
	u.Debug = false

	_, peers, dhts := setupDHTS(ctx, 4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Close()
			dhts[i].dialer.(inet.Network).Close()
		}
	}()

	// topology:
	// 0-1, 1-2, 1-3, 2-3
	err := dhts[0].Connect(ctx, peers[1])
	if err != nil {
		t.Fatal(err)
	}

	err = dhts[1].Connect(ctx, peers[2])
	if err != nil {
		t.Fatal(err)
	}

	err = dhts[1].Connect(ctx, peers[3])
	if err != nil {
		t.Fatal(err)
	}

	err = dhts[2].Connect(ctx, peers[3])
	if err != nil {
		t.Fatal(err)
	}

	// fmt.Println("0 is", peers[0])
	// fmt.Println("1 is", peers[1])
	// fmt.Println("2 is", peers[2])
	// fmt.Println("3 is", peers[3])

	ctxT, _ := context.WithTimeout(ctx, time.Second)
	pchan, err := dhts[0].FindPeersConnectedToPeer(ctxT, peers[2].ID())
	if err != nil {
		t.Fatal(err)
	}

	// shouldFind := []peer.Peer{peers[1], peers[3]}
	found := []peer.Peer{}
	for nextp := range pchan {
		found = append(found, nextp)
	}

	// fmt.Printf("querying 0 (%s) FindPeersConnectedToPeer 2 (%s)\n", peers[0], peers[2])
	// fmt.Println("should find 1, 3", shouldFind)
	// fmt.Println("found", found)

	// testPeerListsMatch(t, shouldFind, found)

	log.Warning("TestFindPeersConnectedToPeer is not quite correct")
	if len(found) == 0 {
		t.Fatal("didn't find any peers.")
	}
}

func testPeerListsMatch(t *testing.T, p1, p2 []peer.Peer) {

	if len(p1) != len(p2) {
		t.Fatal("did not find as many peers as should have", p1, p2)
	}

	ids1 := make([]string, len(p1))
	ids2 := make([]string, len(p2))

	for i, p := range p1 {
		ids1[i] = p.ID().Pretty()
	}

	for i, p := range p2 {
		ids2[i] = p.ID().Pretty()
	}

	sort.Sort(sort.StringSlice(ids1))
	sort.Sort(sort.StringSlice(ids2))

	for i := range ids1 {
		if ids1[i] != ids2[i] {
			t.Fatal("Didnt find expected peer", ids1[i], ids2)
		}
	}
}

func TestConnectCollision(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

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
			err := dhtA.Connect(ctx, peerB)
			if err != nil {
				t.Fatal(err)
			}
			done <- struct{}{}
		}()
		go func() {
			err := dhtB.Connect(ctx, peerA)
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
