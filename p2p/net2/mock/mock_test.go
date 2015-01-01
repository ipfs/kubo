package mocknet

import (
	"bytes"
	"io"
	"math/rand"
	"sync"
	"testing"

	inet "github.com/jbenet/go-ipfs/p2p/net2"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	testutil "github.com/jbenet/go-ipfs/util/testutil"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

func randPeer(t *testing.T) peer.ID {
	p, err := testutil.RandPeerID()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNetworkSetup(t *testing.T) {

	ctx := context.Background()
	sk1, _, err := testutil.RandKeyPair(512)
	if err != nil {
		t.Fatal(t)
	}
	sk2, _, err := testutil.RandKeyPair(512)
	if err != nil {
		t.Fatal(t)
	}
	sk3, _, err := testutil.RandKeyPair(512)
	if err != nil {
		t.Fatal(t)
	}
	mn := New(ctx)
	// peers := []peer.ID{p1, p2, p3}

	// add peers to mock net

	a1 := testutil.RandLocalTCPAddress()
	a2 := testutil.RandLocalTCPAddress()
	a3 := testutil.RandLocalTCPAddress()

	n1, err := mn.AddPeer(sk1, a1)
	if err != nil {
		t.Fatal(err)
	}
	p1 := n1.LocalPeer()

	n2, err := mn.AddPeer(sk2, a2)
	if err != nil {
		t.Fatal(err)
	}
	p2 := n2.LocalPeer()

	n3, err := mn.AddPeer(sk3, a3)
	if err != nil {
		t.Fatal(err)
	}
	p3 := n3.LocalPeer()

	// check peers and net
	if mn.Net(p1) != n1 {
		t.Error("net for p1.ID != n1")
	}
	if mn.Net(p2) != n2 {
		t.Error("net for p2.ID != n1")
	}
	if mn.Net(p3) != n3 {
		t.Error("net for p3.ID != n1")
	}

	// link p1<-->p2, p1<-->p1, p2<-->p3, p3<-->p2

	l12, err := mn.LinkPeers(p1, p2)
	if err != nil {
		t.Fatal(err)
	}
	if !(l12.Networks()[0] == n1 && l12.Networks()[1] == n2) &&
		!(l12.Networks()[0] == n2 && l12.Networks()[1] == n1) {
		t.Error("l12 networks incorrect")
	}

	l11, err := mn.LinkPeers(p1, p1)
	if err != nil {
		t.Fatal(err)
	}
	if !(l11.Networks()[0] == n1 && l11.Networks()[1] == n1) {
		t.Error("l11 networks incorrect")
	}

	l23, err := mn.LinkPeers(p2, p3)
	if err != nil {
		t.Fatal(err)
	}
	if !(l23.Networks()[0] == n2 && l23.Networks()[1] == n3) &&
		!(l23.Networks()[0] == n3 && l23.Networks()[1] == n2) {
		t.Error("l23 networks incorrect")
	}

	l32, err := mn.LinkPeers(p3, p2)
	if err != nil {
		t.Fatal(err)
	}
	if !(l32.Networks()[0] == n2 && l32.Networks()[1] == n3) &&
		!(l32.Networks()[0] == n3 && l32.Networks()[1] == n2) {
		t.Error("l32 networks incorrect")
	}

	// check things

	links12 := mn.LinksBetweenPeers(p1, p2)
	if len(links12) != 1 {
		t.Errorf("should be 1 link bt. p1 and p2 (found %d)", len(links12))
	}
	if links12[0] != l12 {
		t.Error("links 1-2 should be l12.")
	}

	links11 := mn.LinksBetweenPeers(p1, p1)
	if len(links11) != 1 {
		t.Errorf("should be 1 link bt. p1 and p1 (found %d)", len(links11))
	}
	if links11[0] != l11 {
		t.Error("links 1-1 should be l11.")
	}

	links23 := mn.LinksBetweenPeers(p2, p3)
	if len(links23) != 2 {
		t.Errorf("should be 2 link bt. p2 and p3 (found %d)", len(links23))
	}
	if !((links23[0] == l23 && links23[1] == l32) ||
		(links23[0] == l32 && links23[1] == l23)) {
		t.Error("links 2-3 should be l23 and l32.")
	}

	// unlinking

	if err := mn.UnlinkPeers(p2, p1); err != nil {
		t.Error(err)
	}

	// check only one link affected:

	links12 = mn.LinksBetweenPeers(p1, p2)
	if len(links12) != 0 {
		t.Errorf("should be 0 now...", len(links12))
	}

	links11 = mn.LinksBetweenPeers(p1, p1)
	if len(links11) != 1 {
		t.Errorf("should be 1 link bt. p1 and p1 (found %d)", len(links11))
	}
	if links11[0] != l11 {
		t.Error("links 1-1 should be l11.")
	}

	links23 = mn.LinksBetweenPeers(p2, p3)
	if len(links23) != 2 {
		t.Errorf("should be 2 link bt. p2 and p3 (found %d)", len(links23))
	}
	if !((links23[0] == l23 && links23[1] == l32) ||
		(links23[0] == l32 && links23[1] == l23)) {
		t.Error("links 2-3 should be l23 and l32.")
	}

	// check connecting

	// first, no conns
	if len(n2.Conns()) > 0 || len(n3.Conns()) > 0 {
		t.Error("should have 0 conn. Got: (%d, %d)", len(n2.Conns()), len(n3.Conns()))
	}

	// connect p2->p3
	if _, err := n2.DialPeer(ctx, p3); err != nil {
		t.Error(err)
	}

	if len(n2.Conns()) != 1 || len(n3.Conns()) != 1 {
		t.Errorf("should have (1,1) conn. Got: (%d, %d)", len(n2.Conns()), len(n3.Conns()))
	}

	// p := PrinterTo(os.Stdout)
	// p.NetworkConns(n1)
	// p.NetworkConns(n2)
	// p.NetworkConns(n3)

	// can create a stream 2->3, 3->2,
	if _, err := n2.NewStream(p3); err != nil {
		t.Error(err)
	}
	if _, err := n3.NewStream(p2); err != nil {
		t.Error(err)
	}

	// but not 1->2 nor 2->2 (not linked), nor 1->1 (not connected)
	if _, err := n1.NewStream(p2); err == nil {
		t.Error("should not be able to connect")
	}
	if _, err := n2.NewStream(p2); err == nil {
		t.Error("should not be able to connect")
	}
	if _, err := n1.NewStream(p1); err == nil {
		t.Error("should not be able to connect")
	}

	// connect p1->p1 (should work)
	if _, err := n1.DialPeer(ctx, p1); err != nil {
		t.Error("p1 should be able to dial self.", err)
	}

	// and a stream too
	if _, err := n1.NewStream(p1); err != nil {
		t.Error(err)
	}

	// connect p1->p2
	if _, err := n1.DialPeer(ctx, p2); err == nil {
		t.Error("p1 should not be able to dial p2, not connected...")
	}

	// connect p3->p1
	if _, err := n3.DialPeer(ctx, p1); err == nil {
		t.Error("p3 should not be able to dial p1, not connected...")
	}

	// relink p1->p2

	l12, err = mn.LinkPeers(p1, p2)
	if err != nil {
		t.Fatal(err)
	}
	if !(l12.Networks()[0] == n1 && l12.Networks()[1] == n2) &&
		!(l12.Networks()[0] == n2 && l12.Networks()[1] == n1) {
		t.Error("l12 networks incorrect")
	}

	// should now be able to connect

	// connect p1->p2
	if _, err := n1.DialPeer(ctx, p2); err != nil {
		t.Error(err)
	}

	// and a stream should work now too :)
	if _, err := n2.NewStream(p3); err != nil {
		t.Error(err)
	}

}

func TestStreams(t *testing.T) {

	mn, err := FullMeshConnected(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}

	handler := func(s inet.Stream) {
		b := make([]byte, 4)
		if _, err := io.ReadFull(s, b); err != nil {
			panic(err)
		}
		if !bytes.Equal(b, []byte("beep")) {
			panic("bytes mismatch")
		}
		if _, err := s.Write([]byte("boop")); err != nil {
			panic(err)
		}
		s.Close()
	}

	nets := mn.Nets()
	for _, n := range nets {
		n.SetStreamHandler(handler)
	}

	s, err := nets[0].NewStream(nets[1].LocalPeer())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.Write([]byte("beep")); err != nil {
		panic(err)
	}
	b := make([]byte, 4)
	if _, err := io.ReadFull(s, b); err != nil {
		panic(err)
	}
	if !bytes.Equal(b, []byte("boop")) {
		panic("bytes mismatch 2")
	}

}

func makePinger(st string, n int) func(inet.Stream) {
	return func(s inet.Stream) {
		go func() {
			defer s.Close()

			for i := 0; i < n; i++ {
				b := make([]byte, 4+len(st))
				if _, err := s.Write([]byte("ping" + st)); err != nil {
					panic(err)
				}
				if _, err := io.ReadFull(s, b); err != nil {
					panic(err)
				}
				if !bytes.Equal(b, []byte("pong"+st)) {
					panic("bytes mismatch")
				}
			}
		}()
	}
}

func makePonger(st string) func(inet.Stream) {
	return func(s inet.Stream) {
		go func() {
			defer s.Close()

			for {
				b := make([]byte, 4+len(st))
				if _, err := io.ReadFull(s, b); err != nil {
					if err == io.EOF {
						return
					}
					panic(err)
				}
				if !bytes.Equal(b, []byte("ping"+st)) {
					panic("bytes mismatch")
				}
				if _, err := s.Write([]byte("pong" + st)); err != nil {
					panic(err)
				}
			}
		}()
	}
}

func TestStreamsStress(t *testing.T) {

	mn, err := FullMeshConnected(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}

	nets := mn.Nets()
	for _, n := range nets {
		n.SetStreamHandler(makePonger("pingpong"))
	}

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			from := rand.Intn(len(nets))
			to := rand.Intn(len(nets))
			s, err := nets[from].NewStream(nets[to].LocalPeer())
			if err != nil {
				log.Debugf("%d (%s) %d (%s)", from, nets[from], to, nets[to])
				panic(err)
			}

			log.Infof("%d start pinging", i)
			makePinger("pingpong", rand.Intn(100))(s)
			log.Infof("%d done pinging", i)
		}(i)
	}

	wg.Wait()
}

func TestAdding(t *testing.T) {

	mn := New(context.Background())

	peers := []peer.ID{}
	for i := 0; i < 3; i++ {
		sk, _, err := testutil.RandKeyPair(512)
		if err != nil {
			t.Fatal(err)
		}

		a := testutil.RandLocalTCPAddress()
		n, err := mn.AddPeer(sk, a)
		if err != nil {
			t.Fatal(err)
		}

		peers = append(peers, n.LocalPeer())
	}

	p1 := peers[0]
	p2 := peers[1]

	// link them
	for _, p1 := range peers {
		for _, p2 := range peers {
			if _, err := mn.LinkPeers(p1, p2); err != nil {
				t.Error(err)
			}
		}
	}

	// set the new stream handler on p2
	n2 := mn.Net(p2)
	if n2 == nil {
		t.Fatalf("no network for %s", p2)
	}
	n2.SetStreamHandler(func(s inet.Stream) {
		defer s.Close()

		b := make([]byte, 4)
		if _, err := io.ReadFull(s, b); err != nil {
			panic(err)
		}
		if string(b) != "beep" {
			panic("did not beep!")
		}

		if _, err := s.Write([]byte("boop")); err != nil {
			panic(err)
		}
	})

	// connect p1 to p2
	if _, err := mn.ConnectPeers(p1, p2); err != nil {
		t.Fatal(err)
	}

	// talk to p2
	n1 := mn.Net(p1)
	if n1 == nil {
		t.Fatalf("no network for %s", p1)
	}

	s, err := n1.NewStream(p2)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.Write([]byte("beep")); err != nil {
		t.Error(err)
	}
	b := make([]byte, 4)
	if _, err := io.ReadFull(s, b); err != nil {
		t.Error(err)
	}
	if !bytes.Equal(b, []byte("boop")) {
		t.Error("bytes mismatch 2")
	}

}
