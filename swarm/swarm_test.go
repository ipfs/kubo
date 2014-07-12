package swarm

import (
	"fmt"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	msgio "github.com/jbenet/go-msgio"
	"net"
	"testing"
)

func pingListen(listener *net.TCPListener, peer *peer.Peer) {
	for {
		c, err := listener.Accept()
		if err == nil {
			fmt.Println("accepeted")
			go pong(c, peer)
		}
	}
}

func pong(c net.Conn, peer *peer.Peer) {
	mrw := msgio.NewReadWriter(c)
	for {
		data := make([]byte, 1024)
		n, err := mrw.ReadMsg(data)
		if err != nil {
			fmt.Printf("error %v\n", err)
			return
		}
		if string(data[:n]) != "ping" {
			fmt.Printf("error: didn't receive ping: '%v'\n", data[:n])
			return
		}
		err = mrw.WriteMsg([]byte("pong"))
		if err != nil {
			fmt.Printf("error %v\n", err)
			return
		}
		fmt.Println("pong")
	}
}

func TestSwarm(t *testing.T) {

	swarm := NewSwarm()
	peers := []*peer.Peer{}
	listeners := []*net.Listener{}
	peerNames := map[string]string{
		"11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a30": "/ip4/127.0.0.1/tcp/1234",
		"11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a31": "/ip4/127.0.0.1/tcp/2345",
		"11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a32": "/ip4/127.0.0.1/tcp/3456",
		"11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33": "/ip4/127.0.0.1/tcp/4567",
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
		n, h, err := a.DialArgs()
		if err != nil {
			t.Fatal("error getting dial args from addr")
		}
		listener, err := net.Listen(n, h)
		if err != nil {
			t.Fatal("error setting up listener", err)
		}
		go pingListen(listener.(*net.TCPListener), peer)

		_, err = swarm.Dial(peer)
		if err != nil {
			t.Fatal("error swarm dialing to peer", err)
		}

		// ok done, add it.
		peers = append(peers, peer)
		listeners = append(listeners, &listener)
	}

	for i, p := range peers {
		swarm.Chan.Outgoing <- Message{Peer: p, Data: []byte("ping")}
		fmt.Println("ping", i)
	}

	got := map[u.Key]bool{}
	for _, _ = range peers {
		msg := <-swarm.Chan.Incoming
		fmt.Println("recving", string(msg.Data))
		if string(msg.Data) != "pong" {
			t.Error("unexpected conn output", msg.Data)
		}
		got[msg.Peer.Key()] = true
	}

	if len(peers) != len(got) {
		t.Error("got less messages than sent")
	}

	fmt.Println("closing")
	swarm.Close()
	for _, listener := range listeners {
		(*listener).(*net.TCPListener).Close()
	}
}
