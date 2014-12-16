package mocknet

import (
	"bytes"
	"io"
	"testing"

	inet "github.com/jbenet/go-ipfs/net"
	peer "github.com/jbenet/go-ipfs/peer"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

func TestNetworkSetup(t *testing.T) {

	p1 := testutil.RandPeer()
	p2 := testutil.RandPeer()
	p3 := testutil.RandPeer()
	peers := []peer.Peer{p1, p2, p3}

	nets, err := MakeNetworks(context.Background(), peers)
	if err != nil {
		t.Fatal(err)
	}

	// check things

	if len(nets) != 3 {
		t.Error("nets must be 3")
	}

	for i, n := range nets {
		if n.local != peers[i] {
			t.Error("peer mismatch")
		}

		if len(n.conns) != (len(nets) - 1) {
			t.Error("conn mismatch")
		}

		for _, c := range n.conns {
			if c.remote.local == n.local {
				t.Error("conn to self")
			}

			if c.remote.conns[n.local] == nil {
				t.Error("conn other side fail")
			}
			if c.remote.conns[n.local].remote.local != n.local {
				t.Error("conn other side fail")
			}
		}

	}

}

func TestStreams(t *testing.T) {

	p1 := testutil.RandPeer()
	p2 := testutil.RandPeer()
	p3 := testutil.RandPeer()
	peers := []peer.Peer{p1, p2, p3}

	nets, err := MakeNetworks(context.Background(), peers)
	if err != nil {
		t.Fatal(err)
	}

	nets[1].SetHandler(inet.ProtocolDHT, func(s inet.Stream) {
		go func() {
			b := make([]byte, 4)
			if _, err := io.ReadFull(s, b); err != nil {
				panic(err)
			}
			if !bytes.Equal(b, []byte("beep")) {
				panic("bytes mismatch")
			}
			if _, err := s.Write([]byte("boop")); err != nil {
				panic(err)
			}
			s.Close()
		}()
	})

	s, err := nets[0].NewStream(inet.ProtocolDHT, nets[1].local)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.Write([]byte("beep")); err != nil {
		panic(err)
	}
	b := make([]byte, 4)
	if _, err := io.ReadFull(s, b); err != nil {
		panic(err)
	}
	if !bytes.Equal(b, []byte("boop")) {
		panic("bytes mismatch 2")
	}

}
