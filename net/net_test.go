package net_test

import (
	"testing"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	inet "github.com/jbenet/go-ipfs/net"
)

// TestConnectednessCorrect starts a few networks, connects a few
// and tests Connectedness value is correct.
func TestConnectednessCorrect(t *testing.T) {

	ctx := context.Background()

	nets := make([]inet.Network, 4)
	for i := 0; i < 4; i++ {
		nets[i] = GenNetwork(t, ctx)
	}

	// connect 0-1, 0-2, 0-3, 1-2, 2-3

	dial := func(a, b inet.Network) {
		DivulgeAddresses(b, a)
		if err := a.DialPeer(ctx, b.LocalPeer()); err != nil {
			t.Fatalf("Failed to dial: %s", err)
		}
	}

	dial(nets[0], nets[1])
	dial(nets[0], nets[3])
	dial(nets[1], nets[2])
	dial(nets[3], nets[2])

	// test those connected show up correctly

	testConnectedness := func(a, b inet.Network, c inet.Connectedness) {
		if a.Connectedness(b.LocalPeer()) != c {
			t.Error("%s is connected to %s, but Connectedness incorrect", a, b)
		}

		// test symmetric case
		if b.Connectedness(a.LocalPeer()) != c {
			t.Error("%s is connected to %s, but Connectedness incorrect", a, b)
		}
	}

	// test connected
	testConnectedness(nets[0], nets[1], inet.Connected)
	testConnectedness(nets[0], nets[3], inet.Connected)
	testConnectedness(nets[1], nets[2], inet.Connected)
	testConnectedness(nets[3], nets[2], inet.Connected)

	// test not connected
	testConnectedness(nets[0], nets[2], inet.NotConnected)
	testConnectedness(nets[1], nets[3], inet.NotConnected)

	for _, n := range nets {
		n.Close()
	}
}
