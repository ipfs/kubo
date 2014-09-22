package conn

import (
	"net"
	"testing"

	peer "github.com/jbenet/go-ipfs/peer"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
)

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

func echoListen(listener *net.TCPListener) {
	for {
		c, err := listener.Accept()
		if err == nil {
			// fmt.Println("accepeted")
			go echo(c)
		}
	}
}

func echo(c net.Conn) {
	for {
		data := make([]byte, 1024)
		i, err := c.Read(data)
		if err != nil {
			// fmt.Printf("error %v\n", err)
			return
		}
		_, err = c.Write(data[:i])
		if err != nil {
			// fmt.Printf("error %v\n", err)
			return
		}
		// fmt.Println("echoing", data[:i])
	}
}

func TestDial(t *testing.T) {

	listener, err := net.Listen("tcp", "127.0.0.1:1234")
	if err != nil {
		t.Fatal("error setting up listener", err)
	}
	go echoListen(listener.(*net.TCPListener))

	p, err := setupPeer("11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33", "/ip4/127.0.0.1/tcp/1234")
	if err != nil {
		t.Fatal("error setting up peer", err)
	}

	c, err := Dial("tcp", p)
	if err != nil {
		t.Fatal("error dialing peer", err)
	}

	// fmt.Println("sending")
	c.Outgoing.MsgChan <- []byte("beep")
	c.Outgoing.MsgChan <- []byte("boop")
	out := <-c.Incoming.MsgChan
	// fmt.Println("recving", string(out))
	if string(out) != "beep" {
		t.Error("unexpected conn output")
	}

	out = <-c.Incoming.MsgChan
	if string(out) != "boop" {
		t.Error("unexpected conn output")
	}

	// fmt.Println("closing")
	c.Close()
	listener.Close()
}
