package conn

import (
	"io"
	"net"
	"testing"
	"time"

	tu "github.com/jbenet/go-ipfs/util/testutil"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

func echoListen(ctx context.Context, listener Listener) {
	for {
		c, err := listener.Accept()
		if err != nil {

			select {
			case <-ctx.Done():
				return
			default:
			}

			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				<-time.After(time.Microsecond * 10)
				continue
			}

			log.Debugf("echoListen: listener appears to be closing")
			return
		}

		go echo(c.(Conn))
	}
}

func echo(c Conn) {
	io.Copy(c, c)
}

func setupSecureConn(t *testing.T, ctx context.Context) (a, b Conn, p1, p2 tu.PeerNetParams) {
	return setupConn(t, ctx, true)
}

func setupSingleConn(t *testing.T, ctx context.Context) (a, b Conn, p1, p2 tu.PeerNetParams) {
	return setupConn(t, ctx, false)
}

func setupConn(t *testing.T, ctx context.Context, secure bool) (a, b Conn, p1, p2 tu.PeerNetParams) {

	p1 = tu.RandPeerNetParams(t)
	p2 = tu.RandPeerNetParams(t)
	laddr := p1.Addr

	key1 := p1.PrivKey
	key2 := p2.PrivKey
	if !secure {
		key1 = nil
		key2 = nil
	}
	l1, err := Listen(ctx, laddr, p1.ID, key1)
	if err != nil {
		t.Fatal(err)
	}

	d2 := &Dialer{
		LocalPeer:  p2.ID,
		PrivateKey: key2,
	}

	var c2 Conn

	done := make(chan error)
	go func() {
		var err error
		c2, err = d2.Dial(ctx, p1.Addr, p1.ID)
		if err != nil {
			done <- err
		}
		close(done)
	}()

	c1, err := l1.Accept()
	if err != nil {
		t.Fatal("failed to accept", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	return c1.(Conn), c2, p1, p2
}

func testDialer(t *testing.T, secure bool) {
	// t.Skip("Skipping in favor of another test")

	p1 := tu.RandPeerNetParams(t)
	p2 := tu.RandPeerNetParams(t)

	key1 := p1.PrivKey
	key2 := p2.PrivKey
	if !secure {
		key1 = nil
		key2 = nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	l1, err := Listen(ctx, p1.Addr, p1.ID, key1)
	if err != nil {
		t.Fatal(err)
	}

	d2 := &Dialer{
		LocalPeer:  p2.ID,
		PrivateKey: key2,
	}

	go echoListen(ctx, l1)

	c, err := d2.Dial(ctx, p1.Addr, p1.ID)
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
	l1.Close()
	cancel()
}

func TestDialerInsecure(t *testing.T) {
	// t.Skip("Skipping in favor of another test")
	testDialer(t, false)
}

func TestDialerSecure(t *testing.T) {
	// t.Skip("Skipping in favor of another test")
	testDialer(t, true)
}
