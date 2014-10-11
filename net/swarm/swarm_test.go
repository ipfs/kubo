package swarm

import (
	"fmt"
	"testing"

	msg "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	msgio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr/net"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
)

func pingListen(t *testing.T, listener manet.Listener, peer *peer.Peer) {
	for {
		c, err := listener.Accept()
		if err == nil {
			go pong(t, c, peer)
		}
	}
}

func pong(t *testing.T, c manet.Conn, peer *peer.Peer) {
	mrw := msgio.NewReadWriter(c)
	for {
		data := make([]byte, 1024)
		n, err := mrw.ReadMsg(data)
		if err != nil {
			fmt.Printf("error %v\n", err)
			return
		}
		d := string(data[:n])
		if d != "ping" {
			t.Errorf("error: didn't receive ping: '%v'\n", d)
			return
		}
		err = mrw.WriteMsg([]byte("pong"))
		if err != nil {
			fmt.Printf("error %v\n", err)
			return
		}
	}
}

func setupPeer(id string, addr string) (*peer.Peer, error) {
	tcp, err := ma.NewMultiaddr(addr)
	if err != nil {
		return nil, err
	}

	mh, err := mh.FromHexString(id)
	if err != nil {
		return nil, err
	}

	p := &peer.Peer{ID: peer.ID(mh)}
	p.AddAddress(tcp)
	return p, nil
}

func TestSwarm(t *testing.T) {
	t.Skip("TODO FIXME nil pointer")

	local, err := setupPeer("11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a30",
		"/ip4/127.0.0.1/tcp/1234")
	if err != nil {
		t.Fatal("error setting up peer", err)
	}

	peerstore := peer.NewPeerstore()

	swarm, err := NewSwarm(context.Background(), local, peerstore)
	if err != nil {
		t.Error(err)
	}
	var peers []*peer.Peer
	var listeners []manet.Listener
	peerNames := map[string]string{
		"11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a31": "/ip4/127.0.0.1/tcp/2345",
		"11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a32": "/ip4/127.0.0.1/tcp/3456",
		"11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33": "/ip4/127.0.0.1/tcp/4567",
		"11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a34": "/ip4/127.0.0.1/tcp/5678",
	}

	for k, n := range peerNames {
		peer, err := setupPeer(k, n)
		if err != nil {
			t.Fatal("error setting up peer", err)
		}
		a := peer.NetAddress("tcp")
		if a == nil {
			t.Fatal("error setting up peer (addr is nil)", peer)
		}
		listener, err := manet.Listen(a)
		if err != nil {
			t.Fatal("error setting up listener", err)
		}
		go pingListen(t, listener, peer)

		_, err = swarm.Dial(peer)
		if err != nil {
			t.Fatal("error swarm dialing to peer", err)
		}

		// ok done, add it.
		peers = append(peers, peer)
		listeners = append(listeners, listener)
	}

	MsgNum := 1000
	for k := 0; k < MsgNum; k++ {
		for _, p := range peers {
			swarm.Outgoing <- msg.New(p, []byte("ping"))
		}
	}

	got := map[u.Key]int{}

	for k := 0; k < (MsgNum * len(peers)); k++ {
		msg := <-swarm.Incoming
		if string(msg.Data()) != "pong" {
			t.Error("unexpected conn output", msg.Data)
		}

		n, _ := got[msg.Peer().Key()]
		got[msg.Peer().Key()] = n + 1
	}

	if len(peers) != len(got) {
		t.Error("got less messages than sent")
	}

	for p, n := range got {
		if n != MsgNum {
			t.Error("peer did not get all msgs", p, n, "/", MsgNum)
		}
	}

	swarm.Close()
	for _, listener := range listeners {
		listener.Close()
	}
}
