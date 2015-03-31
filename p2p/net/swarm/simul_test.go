package swarm

import (
	"sync"
	"testing"
	"time"

	peer "github.com/ipfs/go-ipfs/p2p/peer"
	ci "github.com/ipfs/go-ipfs/util/testutil/ci"

	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

func TestSimultOpen(t *testing.T) {
	// t.Skip("skipping for another test")
	t.Parallel()

	ctx := context.Background()
	swarms := makeSwarms(ctx, t, 2)

	// connect everyone
	{
		var wg sync.WaitGroup
		connect := func(s *Swarm, dst peer.ID, addr ma.Multiaddr) {
			// copy for other peer
			log.Debugf("TestSimultOpen: connecting: %s --> %s (%s)", s.local, dst, addr)
			s.peers.AddAddr(dst, addr, peer.PermanentAddrTTL)
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
	rounds := 10
	if ci.IsRunning() {
		addrs = 10
		rounds = 5
	}
	SubtestSwarm(t, addrs, rounds)
}

func TestSimultOpenFewStress(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	// t.Skip("skipping for another test")
	t.Parallel()

	msgs := 40
	swarms := 2
	rounds := 10
	// rounds := 100

	for i := 0; i < rounds; i++ {
		SubtestSwarm(t, swarms, msgs)
		<-time.After(10 * time.Millisecond)
	}
}
