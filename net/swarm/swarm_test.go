package swarm

import (
	"bytes"
	"sync"
	"testing"
	"time"

	ci "github.com/jbenet/go-ipfs/crypto"
	msg "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

func pong(ctx context.Context, swarm *Swarm) {
	i := 0
	for {
		select {
		case <-ctx.Done():
			return
		case m1 := <-swarm.Incoming:
			if bytes.Equal(m1.Data(), []byte("ping")) {
				m2 := msg.New(m1.Peer(), []byte("pong"))
				i++
				log.Debugf("%s pong %s (%d)", swarm.local, m1.Peer(), i)
				swarm.Outgoing <- m2
			}
		}
	}
}

func setupPeer(t *testing.T, addr string) peer.Peer {
	tcp, err := ma.NewMultiaddr(addr)
	if err != nil {
		t.Fatal(err)
	}

	sk, pk, err := ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		t.Fatal(err)
	}

	p, err := peer.WithKeyPair(sk, pk)
	if err != nil {
		t.Fatal(err)
	}
	p.AddAddress(tcp)
	return p
}

func makeSwarms(ctx context.Context, t *testing.T, addrs []string) ([]*Swarm, []peer.Peer) {
	swarms := []*Swarm{}

	for _, addr := range addrs {
		local := setupPeer(t, addr)
		peerstore := peer.NewPeerstore()
		swarm, err := NewSwarm(ctx, local, peerstore)
		if err != nil {
			t.Fatal(err)
		}
		swarms = append(swarms, swarm)
	}

	peers := make([]peer.Peer, len(swarms))
	for i, s := range swarms {
		peers[i] = s.local
	}

	return swarms, peers
}

func SubtestSwarm(t *testing.T, addrs []string, MsgNum int) {
	// t.Skip("skipping for another test")

	ctx := context.Background()
	swarms, peers := makeSwarms(ctx, t, addrs)

	// connect everyone
	{
		var wg sync.WaitGroup
		connect := func(s *Swarm, dst peer.Peer) {
			// copy for other peer

			cp, err := s.peers.Get(dst.ID())
			if err != nil {
				t.Fatal(err)
			}
			cp.AddAddress(dst.Addresses()[0])

			log.Info("SWARM TEST: %s dialing %s", s.local, dst)
			if _, err := s.Dial(cp); err != nil {
				t.Fatal("error swarm dialing to peer", err)
			}
			log.Info("SWARM TEST: %s connected to %s", s.local, dst)
			wg.Done()
		}

		log.Info("Connecting swarms simultaneously.")
		for _, s := range swarms {
			for _, p := range peers {
				if p != s.local { // don't connect to self.
					wg.Add(1)
					connect(s, p)
				}
			}
		}
		wg.Wait()
	}

	// ping/pong
	for _, s1 := range swarms {
		ctx, cancel := context.WithCancel(ctx)

		// setup all others to pong
		for _, s2 := range swarms {
			if s1 == s2 {
				continue
			}

			go pong(ctx, s2)
		}

		peers, err := s1.peers.All()
		if err != nil {
			t.Fatal(err)
		}

		for k := 0; k < MsgNum; k++ {
			for _, p := range *peers {
				log.Debugf("%s ping %s (%d)", s1.local, p, k)
				s1.Outgoing <- msg.New(p, []byte("ping"))
			}
		}

		got := map[u.Key]int{}
		for k := 0; k < (MsgNum * len(*peers)); k++ {
			log.Debugf("%s waiting for pong (%d)", s1.local, k)
			msg := <-s1.Incoming
			if string(msg.Data()) != "pong" {
				t.Error("unexpected conn output", msg.Data)
			}

			n, _ := got[msg.Peer().Key()]
			got[msg.Peer().Key()] = n + 1
		}

		if len(*peers) != len(got) {
			t.Error("got less messages than sent")
		}

		for p, n := range got {
			if n != MsgNum {
				t.Error("peer did not get all msgs", p, n, "/", MsgNum)
			}
		}

		cancel()
		<-time.After(50 * time.Microsecond)
	}

	for _, s := range swarms {
		s.Close()
	}
}

func TestSwarm(t *testing.T) {
	// t.Skip("skipping for another test")

	addrs := []string{
		"/ip4/127.0.0.1/tcp/10234",
		"/ip4/127.0.0.1/tcp/10235",
		"/ip4/127.0.0.1/tcp/10236",
		"/ip4/127.0.0.1/tcp/10237",
		"/ip4/127.0.0.1/tcp/10238",
	}

	// msgs := 1000
	msgs := 100
	SubtestSwarm(t, addrs, msgs)
}
