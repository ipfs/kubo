package peer_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	peer "github.com/jbenet/go-ipfs/p2p/peer"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

func TestLatencyEWMAFun(t *testing.T) {
	t.Skip("run it for fun")

	m := peer.NewMetrics()
	id, err := testutil.RandPeerID()
	if err != nil {
		t.Fatal(err)
	}

	mu := 100.0
	sig := 10.0
	next := func() time.Duration {
		mu = (rand.NormFloat64() * sig) + mu
		return time.Duration(mu)
	}

	print := func() {
		fmt.Printf("%3.f %3.f --> %d\n", sig, mu, m.LatencyEWMA(id))
	}

	for {
		select {
		case <-time.After(200 * time.Millisecond):
			m.RecordLatency(id, next())
			print()
		}
	}
}
