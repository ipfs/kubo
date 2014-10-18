package conn

import (
	"testing"

	ci "github.com/jbenet/go-ipfs/crypto"
	peer "github.com/jbenet/go-ipfs/peer"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

func setupPeer(addr string) (*peer.Peer, error) {
	tcp, err := ma.NewMultiaddr(addr)
	if err != nil {
		return nil, err
	}

	sk, pk, err := ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		return nil, err
	}

	id, err := peer.IDFromPubKey(pk)
	if err != nil {
		return nil, err
	}

	p := &peer.Peer{ID: id}
	p.PrivKey = sk
	p.PubKey = pk
	p.AddAddress(tcp)
	return p, nil
}

func echoListen(ctx context.Context, listener Listener) {
	for {
		select {
		case <-ctx.Done():
			return
		case c := <-listener.Accept():
			go echo(ctx, c)
		}
	}
}

func echo(ctx context.Context, c Conn) {
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-c.In():
			c.Out() <- m
		}
	}
}

func TestDialer(t *testing.T) {

	p1, err := setupPeer("/ip4/127.0.0.1/tcp/1234")
	if err != nil {
		t.Fatal("error setting up peer", err)
	}

	p2, err := setupPeer("/ip4/127.0.0.1/tcp/3456")
	if err != nil {
		t.Fatal("error setting up peer", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	laddr := p1.NetAddress("tcp")
	if laddr == nil {
		t.Fatal("Listen address is nil.")
	}

	l, err := Listen(ctx, laddr, p1, peer.NewPeerstore())
	if err != nil {
		t.Fatal(err)
	}

	go echoListen(ctx, l)

	d := &Dialer{
		Peerstore: peer.NewPeerstore(),
		LocalPeer: p2,
	}

	c, err := d.Dial(ctx, "tcp", p1)
	if err != nil {
		t.Fatal("error dialing peer", err)
	}

	// fmt.Println("sending")
	c.Out() <- []byte("beep")
	c.Out() <- []byte("boop")

	out := <-c.In()
	// fmt.Println("recving", string(out))
	data := string(out)
	if data != "beep" {
		t.Error("unexpected conn output", data)
	}

	out = <-c.In()
	data = string(out)
	if string(out) != "boop" {
		t.Error("unexpected conn output", data)
	}

	// fmt.Println("closing")
	c.Close()
	l.Close()
	cancel()
}
