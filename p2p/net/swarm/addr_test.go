package swarm

import (
	"testing"

	peer "github.com/jbenet/go-ipfs/p2p/peer"
	testutil "github.com/jbenet/go-ipfs/util/testutil"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

func TestFilterAddrs(t *testing.T) {

	m := func(s string) ma.Multiaddr {
		maddr, err := ma.NewMultiaddr(s)
		if err != nil {
			t.Fatal(err)
		}
		return maddr
	}

	bad := []ma.Multiaddr{
		m("/ip4/1.2.3.4/udp/1234"),           // unreliable
		m("/ip4/1.2.3.4/udp/1234/sctp/1234"), // not in manet
		m("/ip4/1.2.3.4/udp/1234/utp"),       // utp is broken
		m("/ip4/1.2.3.4/udp/1234/udt"),       // udt is broken on arm
	}

	good := []ma.Multiaddr{
		m("/ip4/127.0.0.1/tcp/1234"),
		m("/ip6/::1/tcp/1234"),
	}

	goodAndBad := append(good, bad...)

	// test filters

	for _, a := range bad {
		if AddrUsable(a) {
			t.Errorf("addr %s should be unusable", a)
		}
	}

	for _, a := range good {
		if !AddrUsable(a) {
			t.Errorf("addr %s should be usable", a)
		}
	}

	subtestAddrsEqual(t, FilterAddrs(bad), []ma.Multiaddr{})
	subtestAddrsEqual(t, FilterAddrs(good), good)
	subtestAddrsEqual(t, FilterAddrs(goodAndBad), good)

	// now test it with swarm

	id, err := testutil.RandPeerID()
	if err != nil {
		t.Fatal(err)
	}

	ps := peer.NewPeerstore()
	ctx := context.Background()

	if _, err := NewNetwork(ctx, bad, id, ps); err == nil {
		t.Fatal("should have failed to create swarm")
	}

	if _, err := NewNetwork(ctx, good, id, ps); err != nil {
		t.Fatal("should have succeeded in creating swarm", err)
	}

	if _, err := NewNetwork(ctx, goodAndBad, id, ps); err == nil {
		t.Fatal("should have failed to create swarm")
	}
}

func subtestAddrsEqual(t *testing.T, a, b []ma.Multiaddr) {
	if len(a) != len(b) {
		t.Error(t)
	}

	in := func(addr ma.Multiaddr, l []ma.Multiaddr) bool {
		for _, addr2 := range l {
			if addr.Equal(addr2) {
				return true
			}
		}
		return false
	}

	for _, aa := range a {
		if !in(aa, b) {
			t.Errorf("%s not in %s", aa, b)
		}
	}
}
