package swarm

import (
	"sync"
	"testing"
	"time"

	peer "github.com/jbenet/go-ipfs/p2p/peer"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

func TestSimultDials(t *testing.T) {
	// t.Skip("skipping for another test")

	ctx := context.Background()
	swarms := makeSwarms(ctx, t, 2)

	// connect everyone
	{
		var wg sync.WaitGroup
		connect := func(s *Swarm, dst peer.ID, addr ma.Multiaddr) {
			// copy for other peer
			log.Debugf("TestSimultOpen: connecting: %s --> %s (%s)", s.local, dst, addr)
			s.peers.AddAddress(dst, addr)
			if _, err := s.Dial(ctx, dst); err != nil {
				t.Fatal("error swarm dialing to peer", err)
			}
			wg.Done()
		}

		log.Info("Connecting swarms simultaneously.")
		for i := 0; i < 10; i++ { // connect 10x for each.
			wg.Add(2)
			go connect(swarms[0], swarms[1].local, swarms[1].ListenAddresses()[0])
			go connect(swarms[1], swarms[0].local, swarms[0].ListenAddresses()[0])
		}
		wg.Wait()
	}

	// should still just have 1, at most 2 connections :)
	c01l := len(swarms[0].ConnectionsToPeer(swarms[1].local))
	if c01l > 2 {
		t.Error("0->1 has", c01l)
	}
	c10l := len(swarms[1].ConnectionsToPeer(swarms[0].local))
	if c10l > 2 {
		t.Error("1->0 has", c10l)
	}

	for _, s := range swarms {
		s.Close()
	}
}

func TestSimultOpen(t *testing.T) {
	// t.Skip("skipping for another test")

	ctx := context.Background()
	swarms := makeSwarms(ctx, t, 2)

	// connect everyone
	{
		var wg sync.WaitGroup
		connect := func(s *Swarm, dst peer.ID, addr ma.Multiaddr) {
			// copy for other peer
			log.Debugf("TestSimultOpen: connecting: %s --> %s (%s)", s.local, dst, addr)
			s.peers.AddAddress(dst, addr)
			if _, err := s.Dial(ctx, dst); err != nil {
				t.Fatal("error swarm dialing to peer", err)
			}
			wg.Done()
		}

		log.Info("Connecting swarms simultaneously.")
		wg.Add(2)
		go connect(swarms[0], swarms[1].local, swarms[1].ListenAddresses()[0])
		go connect(swarms[1], swarms[0].local, swarms[0].ListenAddresses()[0])
		wg.Wait()
	}

	for _, s := range swarms {
		s.Close()
	}
}

func TestSimultOpenMany(t *testing.T) {
	// t.Skip("very very slow")

	addrs := 20
	SubtestSwarm(t, addrs, 10)
}

func TestSimultOpenFewStress(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	// t.Skip("skipping for another test")

	msgs := 40
	swarms := 2
	rounds := 10
	// rounds := 100

	for i := 0; i < rounds; i++ {
		SubtestSwarm(t, swarms, msgs)
		<-time.After(10 * time.Millisecond)
	}
}
