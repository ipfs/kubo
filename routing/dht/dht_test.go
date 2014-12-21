package dht

import (
	// "fmt"
	"bytes"
	"testing"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dssync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	// ci "github.com/jbenet/go-ipfs/crypto"
	inet "github.com/jbenet/go-ipfs/net"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

func setupDHT(ctx context.Context, t *testing.T, addr ma.Multiaddr) *IpfsDHT {

	sk, pk, err := testutil.RandKeyPair(512)
	if err != nil {
		t.Fatal(err)
	}

	p, err := peer.IDFromPublicKey(pk)
	if err != nil {
		t.Fatal(err)
	}

	peerstore := peer.NewPeerstore()
	peerstore.AddPrivKey(p, sk)
	peerstore.AddPubKey(p, pk)
	peerstore.AddAddress(p, addr)

	n, err := inet.NewNetwork(ctx, []ma.Multiaddr{addr}, p, peerstore)
	if err != nil {
		t.Fatal(err)
	}

	dss := dssync.MutexWrap(ds.NewMapDatastore())
	d := NewDHT(ctx, p, peerstore, n, dss)

	d.Validators["v"] = func(u.Key, []byte) error {
		return nil
	}
	return d
}

func setupDHTS(ctx context.Context, n int, t *testing.T) ([]ma.Multiaddr, []peer.ID, []*IpfsDHT) {
	addrs := make([]ma.Multiaddr, n)
	dhts := make([]*IpfsDHT, n)
	peers := make([]peer.ID, n)

	for i := 0; i < n; i++ {
		addrs[i] = testutil.RandLocalTCPAddress()
		dhts[i] = setupDHT(ctx, t, addrs[i])
		peers[i] = dhts[i].self
	}

	return addrs, peers, dhts
}

func connect(t *testing.T, ctx context.Context, a, b *IpfsDHT) {

	idB := b.self
	addrB := b.peerstore.Addresses(idB)
	if len(addrB) == 0 {
		t.Fatal("peers setup incorrectly: no local address")
	}

	a.peerstore.AddAddresses(idB, addrB)
	if err := a.Connect(ctx, idB); err != nil {
		t.Fatal(err)
	}
}

func TestPing(t *testing.T) {
	// t.Skip("skipping test to debug another")
	ctx := context.Background()

	addrA := testutil.RandLocalTCPAddress()
	addrB := testutil.RandLocalTCPAddress()

	dhtA := setupDHT(ctx, t, addrA)
	dhtB := setupDHT(ctx, t, addrB)

	peerA := dhtA.self
	peerB := dhtB.self

	defer dhtA.Close()
	defer dhtB.Close()
	defer dhtA.network.Close()
	defer dhtB.network.Close()

	connect(t, ctx, dhtA, dhtB)

	//Test that we can ping the node
	ctxT, _ := context.WithTimeout(ctx, 100*time.Millisecond)
	if err := dhtA.Ping(ctxT, peerB); err != nil {
		t.Fatal(err)
	}

	ctxT, _ = context.WithTimeout(ctx, 100*time.Millisecond)
	if err := dhtB.Ping(ctxT, peerA); err != nil {
		t.Fatal(err)
	}
}

func TestValueGetSet(t *testing.T) {
	// t.Skip("skipping test to debug another")

	ctx := context.Background()

	addrA := testutil.RandLocalTCPAddress()
	addrB := testutil.RandLocalTCPAddress()

	dhtA := setupDHT(ctx, t, addrA)
	dhtB := setupDHT(ctx, t, addrB)

	defer dhtA.Close()
	defer dhtB.Close()
	defer dhtA.network.Close()
	defer dhtB.network.Close()

	vf := func(u.Key, []byte) error {
		return nil
	}
	dhtA.Validators["v"] = vf
	dhtB.Validators["v"] = vf

	connect(t, ctx, dhtA, dhtB)

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

	_, _, dhts := setupDHTS(ctx, 4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Close()
			defer dhts[i].network.Close()
		}
	}()

	connect(t, ctx, dhts[0], dhts[1])
	connect(t, ctx, dhts[1], dhts[2])
	connect(t, ctx, dhts[1], dhts[3])

	err := dhts[3].putLocal(u.Key("hello"), []byte("world"))
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

	// what is this timeout for? was 60ms before.
	time.Sleep(time.Millisecond * 6)

	ctxT, _ := context.WithTimeout(ctx, time.Second)
	provchan := dhts[0].FindProvidersAsync(ctxT, u.Key("hello"), 1)

	select {
	case prov := <-provchan:
		if prov.ID == "" {
			t.Fatal("Got back nil provider")
		}
		if prov.ID != dhts[3].self {
			t.Fatal("Got back nil provider")
		}
	case <-ctxT.Done():
		t.Fatal("Did not get a provider back.")
	}
}

/*
func TestProvidesAsync(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	ctx := context.Background()

	_, peers, dhts := setupDHTS(ctx, 4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Close()
			defer dhts[i].network.Close()
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

	_, peers, dhts := setupDHTS(ctx, 4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Close()
			defer dhts[i].network.Close()
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

	_, peers, dhts := setupDHTS(ctx, 4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Close()
			dhts[i].network.Close()
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
	t.Skip("not quite correct (see note)")

	if testing.Short() {
		t.SkipNow()
	}

	ctx := context.Background()

	_, peers, dhts := setupDHTS(ctx, 4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Close()
			dhts[i].network.Close()
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

	// shouldFind := []peer.ID{peers[1], peers[3]}
	found := []peer.ID{}
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

func testPeerListsMatch(t *testing.T, p1, p2 []peer.ID) {

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

		addrA := randMultiaddr(t)
		addrB := randMultiaddr(t)

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
		dhtA.network.Close()
		dhtB.network.Close()

		<-time.After(200 * time.Millisecond)
	}
}
*/
