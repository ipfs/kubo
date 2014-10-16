package swarm

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	ci "github.com/jbenet/go-ipfs/crypto"
	msg "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
)

func pong(ctx context.Context, swarm *Swarm) {
	for {
		select {
		case <-ctx.Done():
			return
		case m1 := <-swarm.Incoming:
			if bytes.Equal(m1.Data(), []byte("ping")) {
				m2 := msg.New(m1.Peer(), []byte("pong"))
				swarm.Outgoing <- m2
			}
		}
	}
}

func setupPeer(t *testing.T, id string, addr string) *peer.Peer {
	tcp, err := ma.NewMultiaddr(addr)
	if err != nil {
		t.Fatal(err)
	}

	mh, err := mh.FromHexString(id)
	if err != nil {
		t.Fatal(err)
	}

	p := &peer.Peer{ID: peer.ID(mh)}

	sk, pk, err := ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		t.Fatal(err)
	}
	p.PrivKey = sk
	p.PubKey = pk

	p.AddAddress(tcp)
	return p
}

func makeSwarms(ctx context.Context, t *testing.T, peers map[string]string) []*Swarm {
	swarms := []*Swarm{}

	for key, addr := range peers {
		local := setupPeer(t, key, addr)
		peerstore := peer.NewPeerstore()
		swarm, err := NewSwarm(ctx, local, peerstore)
		if err != nil {
			t.Fatal(err)
		}
		swarms = append(swarms, swarm)
	}

	return swarms
}

func TestSwarm(t *testing.T) {
	peers := map[string]string{
		"11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a30": "/ip4/127.0.0.1/tcp/1234",
		"11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a31": "/ip4/127.0.0.1/tcp/2345",
		"11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a32": "/ip4/127.0.0.1/tcp/3456",
		"11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33": "/ip4/127.0.0.1/tcp/4567",
		"11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a34": "/ip4/127.0.0.1/tcp/5678",
	}

	ctx := context.Background()
	swarms := makeSwarms(ctx, t, peers)

	// connect everyone
	for _, s := range swarms {
		peers, err := s.peers.All()
		if err != nil {
			t.Fatal(err)
		}

		for _, p := range *peers {
			fmt.Println("dialing")
			if _, err := s.Dial(p); err != nil {
				t.Fatal("error swarm dialing to peer", err)
			}
			fmt.Println("dialed")
		}
	}

	// ping/pong
	for _, s1 := range swarms {
		ctx, cancel := context.WithCancel(ctx)

		// setup all others to pong
		for _, s2 := range swarms {
			if s1 == s2 {
				continue
			}

			go pong(ctx, s2)
		}

		peers, err := s1.peers.All()
		if err != nil {
			t.Fatal(err)
		}

		MsgNum := 1000
		for k := 0; k < MsgNum; k++ {
			for _, p := range *peers {
				s1.Outgoing <- msg.New(p, []byte("ping"))
			}
		}

		got := map[u.Key]int{}
		for k := 0; k < (MsgNum * len(*peers)); k++ {
			msg := <-s1.Incoming
			if string(msg.Data()) != "pong" {
				t.Error("unexpected conn output", msg.Data)
			}

			n, _ := got[msg.Peer().Key()]
			got[msg.Peer().Key()] = n + 1
		}

		if len(*peers) != len(got) {
			t.Error("got less messages than sent")
		}

		for p, n := range got {
			if n != MsgNum {
				t.Error("peer did not get all msgs", p, n, "/", MsgNum)
			}
		}

		cancel()
		<-time.After(50 * time.Millisecond)
	}

	for _, s := range swarms {
		s.Close()
	}
}
