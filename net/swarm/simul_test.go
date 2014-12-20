package swarm

import (
	"fmt"
	"sync"
	"testing"

	peer "github.com/jbenet/go-ipfs/peer"
	"github.com/jbenet/go-ipfs/util/testutil"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

func TestSimultOpen(t *testing.T) {
	// t.Skip("skipping for another test")

	addrs := []string{
		"/ip4/127.0.0.1/tcp/1244",
		"/ip4/127.0.0.1/tcp/1245",
	}

	ctx := context.Background()
	swarms, _ := makeSwarms(ctx, t, addrs)

	// connect everyone
	{
		var wg sync.WaitGroup
		connect := func(s *Swarm, dst peer.ID) {
			// copy for other peer
			cp := testutil.NewPeerWithID(dst.ID())
			cp.AddAddress(dst.Addresses()[0])

			if _, err := s.Dial(ctx, cp); err != nil {
				t.Fatal("error swarm dialing to peer", err)
			}
			wg.Done()
		}

		log.Info("Connecting swarms simultaneously.")
		wg.Add(2)
		go connect(swarms[0], swarms[1].local)
		go connect(swarms[1], swarms[0].local)
		wg.Wait()
	}

	for _, s := range swarms {
		s.Close()
	}
}

func TestSimultOpenMany(t *testing.T) {
	// t.Skip("very very slow")

	many := 10
	addrs := []string{}
	for i := 2200; i < (2200 + many); i++ {
		s := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", i)
		addrs = append(addrs, s)
	}

	SubtestSwarm(t, addrs, 10)
}

func TestSimultOpenFewStress(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	// t.Skip("skipping for another test")

	num := 10
	// num := 100
	for i := 0; i < num; i++ {
		addrs := []string{
			fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", 1900+i),
			fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", 2900+i),
		}

		SubtestSwarm(t, addrs, 10)
	}
}
