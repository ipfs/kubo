package net_test

import (
	"testing"
	"time"

	inet "github.com/jbenet/go-ipfs/net"
	handshake "github.com/jbenet/go-ipfs/net/handshake"
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

	n1p := n1.LocalPeer()
	n2p := n2.LocalPeer()

	testKnowsAddrs(t, n1, n2p, []ma.Multiaddr{}) // nothing
	testKnowsAddrs(t, n2, n1p, []ma.Multiaddr{}) // nothing

	// have n2 tell n1, so we can dial...
	DivulgeAddresses(n2, n1)

	testKnowsAddrs(t, n1, n2p, n2.Peerstore().Addresses(n2p)) // has them
	testKnowsAddrs(t, n2, n1p, []ma.Multiaddr{})              // nothing

	if err := n1.DialPeer(ctx, n2p); err != nil {
		t.Fatalf("Failed to dial:", err)
	}

	// this is shitty. dial should wait for connecting to end
	<-time.After(100 * time.Millisecond)

	// the IDService should be opened automatically, by the network.
	// what we should see now is that both peers know about each others listen addresses.
	testKnowsAddrs(t, n1, n2p, n2.Peerstore().Addresses(n2p)) // has them
	testKnowsAddrs(t, n2, n1p, n1.Peerstore().Addresses(n1p)) // has them

	// and the protocol versions.
	testHasProtocolVersions(t, n1, n2p)
	testHasProtocolVersions(t, n2, n1p)
}

func testKnowsAddrs(t *testing.T, n inet.Network, p peer.ID, expected []ma.Multiaddr) {
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

func testHasProtocolVersions(t *testing.T, n inet.Network, p peer.ID) {
	v, err := n.Peerstore().Get(p, "ProtocolVersion")
	if v.(string) != handshake.IpfsVersion.String() {
		t.Fatal("protocol mismatch", err)
	}
	v, err = n.Peerstore().Get(p, "AgentVersion")
	if v.(string) != handshake.ClientVersion {
		t.Fatal("agent version mismatch", err)
	}
}
