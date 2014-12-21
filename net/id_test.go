package net_test

import (
	"testing"
	"time"

	inet "github.com/jbenet/go-ipfs/net"
	peer "github.com/jbenet/go-ipfs/peer"
	testutil "github.com/jbenet/go-ipfs/util/testutil"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

func GenNetwork(t *testing.T, ctx context.Context) inet.Network {
	p := testutil.RandPeerNetParams(t)
	ps := peer.NewPeerstore()
	ps.AddAddress(p.ID, p.Addr)
	ps.AddPubKey(p.ID, p.PubKey)
	ps.AddPrivKey(p.ID, p.PrivKey)
	n, err := inet.NewNetwork(ctx, ps.Addresses(p.ID), p.ID, ps)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func DivulgeAddresses(a, b inet.Network) {
	id := a.LocalPeer()
	addrs := a.Peerstore().Addresses(id)
	b.Peerstore().AddAddresses(id, addrs)
}

func TestIDService(t *testing.T) {

	// the generated networks should have the id service wired in.
	ctx := context.Background()
	n1 := GenNetwork(t, ctx)
	n2 := GenNetwork(t, ctx)

	testKnowsAddrs := func(n inet.Network, p peer.ID, expected []ma.Multiaddr) {
		actual := n.Peerstore().Addresses(p)

		if len(actual) != len(expected) {
			t.Error("dont have the same addresses")
		}

		have := map[string]struct{}{}
		for _, addr := range actual {
			have[addr.String()] = struct{}{}
		}
		for _, addr := range expected {
			if _, found := have[addr.String()]; !found {
				t.Errorf("%s did not have addr for %s: %s", n.LocalPeer(), p, addr)
				panic("ahhhhhhh")
			}
		}
	}

	n1p := n1.LocalPeer()
	n2p := n2.LocalPeer()

	testKnowsAddrs(n1, n2p, []ma.Multiaddr{}) // nothing
	testKnowsAddrs(n2, n1p, []ma.Multiaddr{}) // nothing

	// have n2 tell n1, so we can dial...
	DivulgeAddresses(n2, n1)

	testKnowsAddrs(n1, n2p, n2.Peerstore().Addresses(n2p)) // has them
	testKnowsAddrs(n2, n1p, []ma.Multiaddr{})              // nothing

	if err := n1.DialPeer(ctx, n2p); err != nil {
		t.Fatalf("Failed to dial:", err)
	}

	<-time.After(100 * time.Millisecond)

	// the IDService should be opened automatically, by the network.
	// what we should see now is that both peers know about each others listen addresses.
	testKnowsAddrs(n1, n2p, n2.Peerstore().Addresses(n2p)) // has them
	testKnowsAddrs(n2, n1p, n1.Peerstore().Addresses(n1p)) // has them
}
