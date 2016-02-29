package conn

import (
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	ma "gx/ipfs/QmR3JkmZBKYXgNMNsNZawm914455Qof3PEopwuVSeXG7aV/go-multiaddr"
	ic "gx/ipfs/QmUBogf4nUefBjmYjn6jfsfPJRkmDGSeMhNj4usRKq69f4/go-libp2p/p2p/crypto"
	transport "gx/ipfs/QmUBogf4nUefBjmYjn6jfsfPJRkmDGSeMhNj4usRKq69f4/go-libp2p/p2p/net/transport"
	peer "gx/ipfs/QmUBogf4nUefBjmYjn6jfsfPJRkmDGSeMhNj4usRKq69f4/go-libp2p/p2p/peer"
	tu "gx/ipfs/QmUBogf4nUefBjmYjn6jfsfPJRkmDGSeMhNj4usRKq69f4/go-libp2p/testutil"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
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

func Listen(ctx context.Context, addr ma.Multiaddr, local peer.ID, sk ic.PrivKey) (Listener, error) {
	list, err := transport.NewTCPTransport().Listen(addr)
	if err != nil {
		return nil, err
	}

	return WrapTransportListener(ctx, list, local, sk)
}

func dialer(t *testing.T, a ma.Multiaddr) transport.Dialer {
	tpt := transport.NewTCPTransport()
	tptd, err := tpt.Dialer(a)
	if err != nil {
		t.Fatal(err)
	}

	return tptd
}

func setupConn(t *testing.T, ctx context.Context, secure bool) (a, b Conn, p1, p2 tu.PeerNetParams) {

	p1 = tu.RandPeerNetParamsOrFatal(t)
	p2 = tu.RandPeerNetParamsOrFatal(t)

	key1 := p1.PrivKey
	key2 := p2.PrivKey
	if !secure {
		key1 = nil
		key2 = nil
	}
	l1, err := Listen(ctx, p1.Addr, p1.ID, key1)
	if err != nil {
		t.Fatal(err)
	}
	p1.Addr = l1.Multiaddr() // Addr has been determined by kernel.

	d2 := &Dialer{
		LocalPeer:  p2.ID,
		PrivateKey: key2,
	}

	d2.AddDialer(dialer(t, p2.Addr))

	var c2 Conn

	done := make(chan error)
	go func() {
		defer close(done)

		var err error
		c2, err = d2.Dial(ctx, p1.Addr, p1.ID)
		if err != nil {
			done <- err
			return
		}

		// if secure, need to read + write, as that's what triggers the handshake.
		if secure {
			if err := sayHello(c2); err != nil {
				done <- err
			}
		}
	}()

	c1, err := l1.Accept()
	if err != nil {
		t.Fatal("failed to accept", err)
	}

	// if secure, need to read + write, as that's what triggers the handshake.
	if secure {
		if err := sayHello(c1); err != nil {
			done <- err
		}
	}

	if err := <-done; err != nil {
		t.Fatal(err)
	}

	return c1.(Conn), c2, p1, p2
}

func sayHello(c net.Conn) error {
	h := []byte("hello")
	if _, err := c.Write(h); err != nil {
		return err
	}
	if _, err := c.Read(h); err != nil {
		return err
	}
	if string(h) != "hello" {
		return fmt.Errorf("did not get hello")
	}
	return nil
}

func testDialer(t *testing.T, secure bool) {
	// t.Skip("Skipping in favor of another test")

	p1 := tu.RandPeerNetParamsOrFatal(t)
	p2 := tu.RandPeerNetParamsOrFatal(t)

	key1 := p1.PrivKey
	key2 := p2.PrivKey
	if !secure {
		key1 = nil
		key2 = nil
		t.Log("testing insecurely")
	} else {
		t.Log("testing securely")
	}

	ctx, cancel := context.WithCancel(context.Background())
	l1, err := Listen(ctx, p1.Addr, p1.ID, key1)
	if err != nil {
		t.Fatal(err)
	}
	p1.Addr = l1.Multiaddr() // Addr has been determined by kernel.

	d2 := &Dialer{
		LocalPeer:  p2.ID,
		PrivateKey: key2,
	}
	d2.AddDialer(dialer(t, p2.Addr))

	go echoListen(ctx, l1)

	c, err := d2.Dial(ctx, p1.Addr, p1.ID)
	if err != nil {
		t.Fatal("error dialing peer", err)
	}

	// fmt.Println("sending")
	mc := msgioWrap(c)
	mc.WriteMsg([]byte("beep"))
	mc.WriteMsg([]byte("boop"))
	out, err := mc.ReadMsg()
	if err != nil {
		t.Fatal(err)
	}

	// fmt.Println("recving", string(out))
	data := string(out)
	if data != "beep" {
		t.Error("unexpected conn output", data)
	}

	out, err = mc.ReadMsg()
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

func testDialerCloseEarly(t *testing.T, secure bool) {
	// t.Skip("Skipping in favor of another test")

	p1 := tu.RandPeerNetParamsOrFatal(t)
	p2 := tu.RandPeerNetParamsOrFatal(t)

	key1 := p1.PrivKey
	if !secure {
		key1 = nil
		t.Log("testing insecurely")
	} else {
		t.Log("testing securely")
	}

	ctx, cancel := context.WithCancel(context.Background())
	l1, err := Listen(ctx, p1.Addr, p1.ID, key1)
	if err != nil {
		t.Fatal(err)
	}
	p1.Addr = l1.Multiaddr() // Addr has been determined by kernel.

	// lol nesting
	d2 := &Dialer{
		LocalPeer: p2.ID,
		// PrivateKey: key2, -- dont give it key. we'll just close the conn.
	}
	d2.AddDialer(dialer(t, p2.Addr))

	errs := make(chan error, 100)
	done := make(chan struct{}, 1)
	gotclosed := make(chan struct{}, 1)
	go func() {
		defer func() { done <- struct{}{} }()

		c, err := l1.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				gotclosed <- struct{}{}
				return
			}
			errs <- err
		}

		if _, err := c.Write([]byte("hello")); err != nil {
			gotclosed <- struct{}{}
			return
		}

		errs <- fmt.Errorf("wrote to conn")
	}()

	c, err := d2.Dial(ctx, p1.Addr, p1.ID)
	if err != nil {
		t.Fatal(err)
	}
	c.Close() // close it early.

	readerrs := func() {
		for {
			select {
			case e := <-errs:
				t.Error(e)
			default:
				return
			}
		}
	}
	readerrs()

	l1.Close()
	<-done
	cancel()
	readerrs()
	close(errs)

	select {
	case <-gotclosed:
	default:
		t.Error("did not get closed")
	}
}

// we dont do a handshake with singleConn, so cant "close early."
// func TestDialerCloseEarlyInsecure(t *testing.T) {
// 	// t.Skip("Skipping in favor of another test")
// 	testDialerCloseEarly(t, false)
// }

func TestDialerCloseEarlySecure(t *testing.T) {
	// t.Skip("Skipping in favor of another test")
	testDialerCloseEarly(t, true)
}
