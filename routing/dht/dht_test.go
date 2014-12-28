package dht

import (
	"bytes"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dssync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	inet "github.com/jbenet/go-ipfs/net"
	peer "github.com/jbenet/go-ipfs/peer"
	routing "github.com/jbenet/go-ipfs/routing"
	u "github.com/jbenet/go-ipfs/util"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

var testCaseValues = map[u.Key][]byte{}

func init() {
	testCaseValues["hello"] = []byte("world")
	for i := 0; i < 100; i++ {
		k := fmt.Sprintf("%d -- key", i)
		v := fmt.Sprintf("%d -- value", i)
		testCaseValues[u.Key(k)] = []byte(v)
	}
}

func setupDHT(ctx context.Context, t *testing.T, addr ma.Multiaddr, seed int64) *IpfsDHT {

	sk, pk, err := testutil.SeededKeyPair(512, seed)
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
	d := NewDHT(ctx, p, n, dss)

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
		dhts[i] = setupDHT(ctx, t, addrs[i], int64(i))
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

func bootstrap(t *testing.T, ctx context.Context, dhts []*IpfsDHT) {

	ctx, cancel := context.WithCancel(ctx)

	rounds := 1
	for i := 0; i < rounds; i++ {
		log.Debugf("bootstrapping round %d/%d\n", i, rounds)

		// tried async. sequential fares much better. compare:
		// 100 async https://gist.github.com/jbenet/56d12f0578d5f34810b2
		// 100 sync https://gist.github.com/jbenet/6c59e7c15426e48aaedd
		// probably because results compound
		for _, dht := range dhts {
			log.Debugf("bootstrapping round %d/%d -- %s\n", i, rounds, dht.self)
			dht.Bootstrap(ctx, 3)
		}
	}

	cancel()
}

func TestPing(t *testing.T) {
	// t.Skip("skipping test to debug another")
	ctx := context.Background()

	addrA := testutil.RandLocalTCPAddress()
	addrB := testutil.RandLocalTCPAddress()

	dhtA := setupDHT(ctx, t, addrA, 1)
	dhtB := setupDHT(ctx, t, addrB, 2)

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

	dhtA := setupDHT(ctx, t, addrA, 1)
	dhtB := setupDHT(ctx, t, addrB, 2)

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

	for k, v := range testCaseValues {
		log.Debugf("adding local values for %s = %s", k, v)
		err := dhts[3].putLocal(k, v)
		if err != nil {
			t.Fatal(err)
		}

		bits, err := dhts[3].getLocal(k)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(bits, v) {
			t.Fatal("didn't store the right bits (%s, %s)", k, v)
		}
	}

	for k, _ := range testCaseValues {
		log.Debugf("announcing provider for %s", k)
		if err := dhts[3].Provide(ctx, k); err != nil {
			t.Fatal(err)
		}
	}

	// what is this timeout for? was 60ms before.
	time.Sleep(time.Millisecond * 6)

	n := 0
	for k, _ := range testCaseValues {
		n = (n + 1) % 3

		log.Debugf("getting providers for %s from %d", k, n)
		ctxT, _ := context.WithTimeout(ctx, time.Second)
		provchan := dhts[n].FindProvidersAsync(ctxT, k, 1)

		select {
		case prov := <-provchan:
			if prov.ID == "" {
				t.Fatal("Got back nil provider")
			}
			if prov.ID != dhts[3].self {
				t.Fatal("Got back wrong provider")
			}
		case <-ctxT.Done():
			t.Fatal("Did not get a provider back.")
		}
	}
}

func TestBootstrap(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	ctx := context.Background()

	nDHTs := 15
	_, _, dhts := setupDHTS(ctx, nDHTs, t)
	defer func() {
		for i := 0; i < nDHTs; i++ {
			dhts[i].Close()
			defer dhts[i].network.Close()
		}
	}()

	t.Logf("connecting %d dhts in a ring", nDHTs)
	for i := 0; i < nDHTs; i++ {
		connect(t, ctx, dhts[i], dhts[(i+1)%len(dhts)])
	}

	<-time.After(100 * time.Millisecond)
	t.Logf("bootstrapping them so they find each other", nDHTs)
	ctxT, _ := context.WithTimeout(ctx, 5*time.Second)
	bootstrap(t, ctxT, dhts)

	if u.Debug {
		// the routing tables should be full now. let's inspect them.
		<-time.After(5 * time.Second)
		t.Logf("checking routing table of %d", nDHTs)
		for _, dht := range dhts {
			fmt.Printf("checking routing table of %s\n", dht.self)
			dht.routingTable.Print()
			fmt.Println("")
		}
	}

	// test "well-formed-ness" (>= 3 peers in every routing table)
	for _, dht := range dhts {
		rtlen := dht.routingTable.Size()
		if rtlen < 4 {
			t.Errorf("routing table for %s only has %d peers", dht.self, rtlen)
		}
	}
}

func TestProvidesMany(t *testing.T) {
	t.Skip("this test doesn't work")
	// t.Skip("skipping test to debug another")
	ctx := context.Background()

	nDHTs := 40
	_, _, dhts := setupDHTS(ctx, nDHTs, t)
	defer func() {
		for i := 0; i < nDHTs; i++ {
			dhts[i].Close()
			defer dhts[i].network.Close()
		}
	}()

	t.Logf("connecting %d dhts in a ring", nDHTs)
	for i := 0; i < nDHTs; i++ {
		connect(t, ctx, dhts[i], dhts[(i+1)%len(dhts)])
	}

	<-time.After(100 * time.Millisecond)
	t.Logf("bootstrapping them so they find each other", nDHTs)
	ctxT, _ := context.WithTimeout(ctx, 5*time.Second)
	bootstrap(t, ctxT, dhts)

	if u.Debug {
		// the routing tables should be full now. let's inspect them.
		<-time.After(5 * time.Second)
		t.Logf("checking routing table of %d", nDHTs)
		for _, dht := range dhts {
			fmt.Printf("checking routing table of %s\n", dht.self)
			dht.routingTable.Print()
			fmt.Println("")
		}
	}

	var providers = map[u.Key]peer.ID{}

	d := 0
	for k, v := range testCaseValues {
		d = (d + 1) % len(dhts)
		dht := dhts[d]
		providers[k] = dht.self

		t.Logf("adding local values for %s = %s (on %s)", k, v, dht.self)
		err := dht.putLocal(k, v)
		if err != nil {
			t.Fatal(err)
		}

		bits, err := dht.getLocal(k)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(bits, v) {
			t.Fatal("didn't store the right bits (%s, %s)", k, v)
		}

		t.Logf("announcing provider for %s", k)
		if err := dht.Provide(ctx, k); err != nil {
			t.Fatal(err)
		}
	}

	// what is this timeout for? was 60ms before.
	time.Sleep(time.Millisecond * 6)

	errchan := make(chan error)

	ctxT, _ = context.WithTimeout(ctx, 5*time.Second)

	var wg sync.WaitGroup
	getProvider := func(dht *IpfsDHT, k u.Key) {
		defer wg.Done()

		expected := providers[k]

		provchan := dht.FindProvidersAsync(ctxT, k, 1)
		select {
		case prov := <-provchan:
			actual := prov.ID
			if actual == "" {
				errchan <- fmt.Errorf("Got back nil provider (%s at %s)", k, dht.self)
			} else if actual != expected {
				errchan <- fmt.Errorf("Got back wrong provider (%s != %s) (%s at %s)",
					expected, actual, k, dht.self)
			}
		case <-ctxT.Done():
			errchan <- fmt.Errorf("Did not get a provider back (%s at %s)", k, dht.self)
		}
	}

	for k, _ := range testCaseValues {
		// everyone should be able to find it...
		for _, dht := range dhts {
			log.Debugf("getting providers for %s at %s", k, dht.self)
			wg.Add(1)
			go getProvider(dht, k)
		}
	}

	// we need this because of printing errors
	go func() {
		wg.Wait()
		close(errchan)
	}()

	for err := range errchan {
		t.Error(err)
	}
}

func TestProvidesAsync(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

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

	time.Sleep(time.Millisecond * 60)

	ctxT, _ := context.WithTimeout(ctx, time.Millisecond*300)
	provs := dhts[0].FindProvidersAsync(ctxT, u.Key("hello"), 5)
	select {
	case p, ok := <-provs:
		if !ok {
			t.Fatal("Provider channel was closed...")
		}
		if p.ID == "" {
			t.Fatal("Got back nil provider!")
		}
		if p.ID != dhts[3].self {
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

	err := dhts[3].Provide(ctx, u.Key("/v/hello"))
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond * 6)

	t.Log("interface was changed. GetValue should not use providers.")
	ctxT, _ := context.WithTimeout(ctx, time.Second)
	val, err := dhts[0].GetValue(ctxT, u.Key("/v/hello"))
	if err != routing.ErrNotFound {
		t.Error(err)
	}
	if string(val) == "world" {
		t.Error("should not get value.")
	}
	if len(val) > 0 && string(val) != "world" {
		t.Error("worse, there's a value and its not even the right one.")
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

	connect(t, ctx, dhts[0], dhts[1])
	connect(t, ctx, dhts[1], dhts[2])
	connect(t, ctx, dhts[1], dhts[3])

	ctxT, _ := context.WithTimeout(ctx, time.Second)
	p, err := dhts[0].FindPeer(ctxT, peers[2])
	if err != nil {
		t.Fatal(err)
	}

	if p.ID == "" {
		t.Fatal("Failed to find peer.")
	}

	if p.ID != peers[2] {
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
	connect(t, ctx, dhts[0], dhts[1])
	connect(t, ctx, dhts[1], dhts[2])
	connect(t, ctx, dhts[1], dhts[3])
	connect(t, ctx, dhts[2], dhts[3])

	// fmt.Println("0 is", peers[0])
	// fmt.Println("1 is", peers[1])
	// fmt.Println("2 is", peers[2])
	// fmt.Println("3 is", peers[3])

	ctxT, _ := context.WithTimeout(ctx, time.Second)
	pchan, err := dhts[0].FindPeersConnectedToPeer(ctxT, peers[2])
	if err != nil {
		t.Fatal(err)
	}

	// shouldFind := []peer.ID{peers[1], peers[3]}
	found := []peer.PeerInfo{}
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
		ids1[i] = string(p)
	}

	for i, p := range p2 {
		ids2[i] = string(p)
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

		addrA := testutil.RandLocalTCPAddress()
		addrB := testutil.RandLocalTCPAddress()

		dhtA := setupDHT(ctx, t, addrA, int64((rtime*2)+1))
		dhtB := setupDHT(ctx, t, addrB, int64((rtime*2)+2))

		peerA := dhtA.self
		peerB := dhtB.self

		errs := make(chan error)
		go func() {
			dhtA.peerstore.AddAddress(peerB, addrB)
			err := dhtA.Connect(ctx, peerB)
			errs <- err
		}()
		go func() {
			dhtB.peerstore.AddAddress(peerA, addrA)
			err := dhtB.Connect(ctx, peerA)
			errs <- err
		}()

		timeout := time.After(time.Second)
		select {
		case e := <-errs:
			if e != nil {
				t.Fatal(e)
			}
		case <-timeout:
			t.Fatal("Timeout received!")
		}
		select {
		case e := <-errs:
			if e != nil {
				t.Fatal(e)
			}
		case <-timeout:
			t.Fatal("Timeout received!")
		}

		dhtA.Close()
		dhtB.Close()
		dhtA.network.Close()
		dhtB.network.Close()
	}
}
