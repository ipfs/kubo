package swarm

import (
	"testing"

	peer "github.com/ipfs/go-ipfs/p2p/peer"

	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

func TestPeers(t *testing.T) {
	// t.Skip("skipping for another test")

	ctx := context.Background()
	swarms := makeSwarms(ctx, t, 2)
	s1 := swarms[0]
	s2 := swarms[1]

	connect := func(s *Swarm, dst peer.ID, addr ma.Multiaddr) {
		// TODO: make a DialAddr func.
		s.peers.AddAddr(dst, addr, peer.PermanentAddrTTL)
		// t.Logf("connections from %s", s.LocalPeer())
		// for _, c := range s.ConnectionsToPeer(dst) {
		// 	t.Logf("connection from %s to %s: %v", s.LocalPeer(), dst, c)
		// }
		// t.Logf("")
		if _, err := s.Dial(ctx, dst); err != nil {
			t.Fatal("error swarm dialing to peer", err)
		}
		// t.Log(s.swarm.Dump())
	}

	s1GotConn := make(chan struct{}, 0)
	s2GotConn := make(chan struct{}, 0)
	s1.SetConnHandler(func(c *Conn) {
		s1GotConn <- struct{}{}
	})
	s2.SetConnHandler(func(c *Conn) {
		s2GotConn <- struct{}{}
	})

	connect(s1, s2.LocalPeer(), s2.ListenAddresses()[0])
	<-s2GotConn // have to wait here so the other side catches up.
	connect(s2, s1.LocalPeer(), s1.ListenAddresses()[0])

	for i := 0; i < 100; i++ {
		connect(s1, s2.LocalPeer(), s2.ListenAddresses()[0])
		connect(s2, s1.LocalPeer(), s1.ListenAddresses()[0])
	}

	for _, s := range swarms {
		log.Infof("%s swarm routing table: %s", s.local, s.Peers())
	}

	test := func(s *Swarm) {
		expect := 1
		actual := len(s.Peers())
		if actual != expect {
			t.Errorf("%s has %d peers, not %d: %v", s.LocalPeer(), actual, expect, s.Peers())
			t.Log(s.swarm.Dump())
		}
		actual = len(s.Connections())
		if actual != expect {
			t.Errorf("%s has %d conns, not %d: %v", s.LocalPeer(), actual, expect, s.Connections())
			t.Log(s.swarm.Dump())
		}
	}

	test(s1)
	test(s2)
}
