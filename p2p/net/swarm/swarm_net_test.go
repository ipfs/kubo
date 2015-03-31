package swarm_test

import (
	"fmt"
	"testing"
	"time"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	inet "github.com/ipfs/go-ipfs/p2p/net"
	testutil "github.com/ipfs/go-ipfs/p2p/test/util"
)

// TestConnectednessCorrect starts a few networks, connects a few
// and tests Connectedness value is correct.
func TestConnectednessCorrect(t *testing.T) {

	ctx := context.Background()

	nets := make([]inet.Network, 4)
	for i := 0; i < 4; i++ {
		nets[i] = testutil.GenSwarmNetwork(t, ctx)
	}

	// connect 0-1, 0-2, 0-3, 1-2, 2-3

	dial := func(a, b inet.Network) {
		testutil.DivulgeAddresses(b, a)
		if _, err := a.DialPeer(ctx, b.LocalPeer()); err != nil {
			t.Fatalf("Failed to dial: %s", err)
		}
	}

	dial(nets[0], nets[1])
	dial(nets[0], nets[3])
	dial(nets[1], nets[2])
	dial(nets[3], nets[2])

	// there's something wrong with dial, i think. it's not finishing
	// completely. there must be some async stuff.
	<-time.After(100 * time.Millisecond)

	// test those connected show up correctly

	// test connected
	expectConnectedness(t, nets[0], nets[1], inet.Connected)
	expectConnectedness(t, nets[0], nets[3], inet.Connected)
	expectConnectedness(t, nets[1], nets[2], inet.Connected)
	expectConnectedness(t, nets[3], nets[2], inet.Connected)

	// test not connected
	expectConnectedness(t, nets[0], nets[2], inet.NotConnected)
	expectConnectedness(t, nets[1], nets[3], inet.NotConnected)

	for _, n := range nets {
		n.Close()
	}
}

func expectConnectedness(t *testing.T, a, b inet.Network, expected inet.Connectedness) {
	es := "%s is connected to %s, but Connectedness incorrect. %s %s"
	if a.Connectedness(b.LocalPeer()) != expected {
		t.Errorf(es, a, b, printConns(a), printConns(b))
	}

	// test symmetric case
	if b.Connectedness(a.LocalPeer()) != expected {
		t.Errorf(es, b, a, printConns(b), printConns(a))
	}
}

func printConns(n inet.Network) string {
	s := fmt.Sprintf("Connections in %s:\n", n)
	for _, c := range n.Conns() {
		s = s + fmt.Sprintf("- %s\n", c)
	}
	return s
}
