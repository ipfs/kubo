package conn

import (
	"io"
	"testing"

	ci "github.com/jbenet/go-ipfs/crypto"
	peer "github.com/jbenet/go-ipfs/peer"
	testutil "github.com/jbenet/go-ipfs/util/testutil"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

func setupPeer(addr string) (peer.Peer, error) {
	tcp, err := ma.NewMultiaddr(addr)
	if err != nil {
		return nil, err
	}

	sk, pk, err := ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		return nil, err
	}

	p, err := testutil.NewPeerWithKeyPair(sk, pk)
	if err != nil {
		return nil, err
	}
	p.AddAddress(tcp)
	return p, nil
}

func echoListen(ctx context.Context, listener Listener) {
	for {
		c, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
		go echo(c.(Conn))
	}
}

func echo(c Conn) {
	io.Copy(c, c)
}

func setupSecureConn(t *testing.T, ctx context.Context, a1, a2 string) (a, b Conn) {
	return setupConn(t, ctx, a1, a2, true)
}

func setupSingleConn(t *testing.T, ctx context.Context, a1, a2 string) (a, b Conn) {
	return setupConn(t, ctx, a1, a2, false)
}

func setupConn(t *testing.T, ctx context.Context, a1, a2 string, secure bool) (a, b Conn) {

	p1, err := setupPeer(a1)
	if err != nil {
		t.Fatal("error setting up peer", err)
	}

	p2, err := setupPeer(a2)
	if err != nil {
		t.Fatal("error setting up peer", err)
	}

	laddr := p1.NetAddress("tcp")
	if laddr == nil {
		t.Fatal("Listen address is nil.")
	}

	ps1 := peer.NewPeerstore()
	ps2 := peer.NewPeerstore()
	ps1.Add(p1)
	ps2.Add(p2)

	l1, err := Listen(ctx, laddr, p1, ps1)
	l1.SetWithoutSecureTransport(!secure)
	if err != nil {
		t.Fatal(err)
	}

	d2 := &Dialer{
		Peerstore:              ps2,
		LocalPeer:              p2,
		WithoutSecureTransport: !secure,
	}

	var c2 Conn

	done := make(chan struct{})
	go func() {
		c2, err = d2.Dial(ctx, "tcp", p1)
		if err != nil {
			t.Fatal("error dialing peer", err)
		}
		done <- struct{}{}
	}()

	c1, err := l1.Accept()
	if err != nil {
		t.Fatal("failed to accept")
	}
	<-done

	return c1.(Conn), c2
}

func TestDialer(t *testing.T) {
	// t.Skip("Skipping in favor of another test")

	p1, err := setupPeer("/ip4/127.0.0.1/tcp/4234")
	if err != nil {
		t.Fatal("error setting up peer", err)
	}

	p2, err := setupPeer("/ip4/127.0.0.1/tcp/4235")
	if err != nil {
		t.Fatal("error setting up peer", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	laddr := p1.NetAddress("tcp")
	if laddr == nil {
		t.Fatal("Listen address is nil.")
	}

	ps1 := peer.NewPeerstore()
	ps2 := peer.NewPeerstore()
	ps1.Add(p1)
	ps2.Add(p2)

	l, err := Listen(ctx, laddr, p1, ps1)
	if err != nil {
		t.Fatal(err)
	}

	go echoListen(ctx, l)

	d := &Dialer{
		Peerstore: ps2,
		LocalPeer: p2,
	}

	c, err := d.Dial(ctx, "tcp", p1)
	if err != nil {
		t.Fatal("error dialing peer", err)
	}

	// fmt.Println("sending")
	c.WriteMsg([]byte("beep"))
	c.WriteMsg([]byte("boop"))

	out, err := c.ReadMsg()
	if err != nil {
		t.Fatal(err)
	}

	// fmt.Println("recving", string(out))
	data := string(out)
	if data != "beep" {
		t.Error("unexpected conn output", data)
	}

	out, err = c.ReadMsg()
	if err != nil {
		t.Fatal(err)
	}

	data = string(out)
	if string(out) != "boop" {
		t.Error("unexpected conn output", data)
	}

	// fmt.Println("closing")
	c.Close()
	l.Close()
	cancel()
}

func TestDialAddr(t *testing.T) {
	// t.Skip("Skipping in favor of another test")

	p1, err := setupPeer("/ip4/127.0.0.1/tcp/4334")
	if err != nil {
		t.Fatal("error setting up peer", err)
	}

	p2, err := setupPeer("/ip4/127.0.0.1/tcp/4335")
	if err != nil {
		t.Fatal("error setting up peer", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	laddr := p1.NetAddress("tcp")
	if laddr == nil {
		t.Fatal("Listen address is nil.")
	}

	ps1 := peer.NewPeerstore()
	ps2 := peer.NewPeerstore()
	ps1.Add(p1)
	ps2.Add(p2)

	l, err := Listen(ctx, laddr, p1, ps1)
	if err != nil {
		t.Fatal(err)
	}

	go echoListen(ctx, l)

	d := &Dialer{
		Peerstore: ps2,
		LocalPeer: p2,
	}

	raddr := p1.NetAddress("tcp")
	if raddr == nil {
		t.Fatal("Dial address is nil.")
	}

	c, err := d.DialAddr(ctx, raddr, p1)
	if err != nil {
		t.Fatal("error dialing peer", err)
	}

	// fmt.Println("sending")
	c.WriteMsg([]byte("beep"))
	c.WriteMsg([]byte("boop"))

	out, err := c.ReadMsg()
	if err != nil {
		t.Fatal(err)
	}
	// fmt.Println("recving", string(out))
	data := string(out)
	if data != "beep" {
		t.Error("unexpected conn output", data)
	}

	out, err = c.ReadMsg()
	if err != nil {
		t.Fatal(err)
	}

	data = string(out)
	if string(out) != "boop" {
		t.Error("unexpected conn output", data)
	}

	// fmt.Println("closing")
	c.Close()
	l.Close()
	cancel()
}
