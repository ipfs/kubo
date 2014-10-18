package conn

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

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

func setupConn(t *testing.T, ctx context.Context, a1, a2 string) (a, b Conn) {

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

	l1, err := Listen(ctx, laddr, p1, peer.NewPeerstore())
	if err != nil {
		t.Fatal(err)
	}

	d2 := &Dialer{
		Peerstore: peer.NewPeerstore(),
		LocalPeer: p2,
	}

	c2, err := d2.Dial(ctx, "tcp", p1)
	if err != nil {
		t.Fatal("error dialing peer", err)
	}

	c1 := <-l1.Accept()

	return c1, c2
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

func TestClose(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	c1, c2 := setupConn(t, ctx, "/ip4/127.0.0.1/tcp/1234", "/ip4/127.0.0.1/tcp/2345")

	select {
	case <-c1.Done():
		t.Fatal("done before close")
	case <-c2.Done():
		t.Fatal("done before close")
	default:
	}

	c1.Close()

	select {
	case <-c1.Done():
	default:
		t.Fatal("not done after cancel")
	}

	c2.Close()

	select {
	case <-c2.Done():
	default:
		t.Fatal("not done after cancel")
	}

	cancel() // close the listener :P
}

func TestCancel(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	c1, c2 := setupConn(t, ctx, "/ip4/127.0.0.1/tcp/1234", "/ip4/127.0.0.1/tcp/2345")

	select {
	case <-c1.Done():
		t.Fatal("done before close")
	case <-c2.Done():
		t.Fatal("done before close")
	default:
	}

	cancel()

	// wait to ensure other goroutines run and close things.
	<-time.After(time.Microsecond * 10)
	// test that cancel called Close.

	select {
	case <-c1.Done():
	default:
		t.Fatal("not done after cancel")
	}

	select {
	case <-c2.Done():
	default:
		t.Fatal("not done after cancel")
	}

}

func TestCloseLeak(t *testing.T) {

	var wg sync.WaitGroup

	runPair := func(p1, p2, num int) {
		a1 := strconv.Itoa(p1)
		a2 := strconv.Itoa(p2)
		ctx, cancel := context.WithCancel(context.Background())
		c1, c2 := setupConn(t, ctx, "/ip4/127.0.0.1/tcp/"+a1, "/ip4/127.0.0.1/tcp/"+a2)

		for i := 0; i < num; i++ {
			b1 := []byte("beep")
			c1.Out() <- b1
			b2 := <-c2.In()
			if !bytes.Equal(b1, b2) {
				panic("bytes not equal")
			}

			b2 = []byte("boop")
			c2.Out() <- b2
			b1 = <-c1.In()
			if !bytes.Equal(b1, b2) {
				panic("bytes not equal")
			}

			<-time.After(time.Microsecond * 5)
		}

		cancel() // close the listener
		wg.Done()
	}

	var cons = 20
	var msgs = 100
	fmt.Printf("Running %d connections * %d msgs.\n", cons, msgs)
	for i := 0; i < cons; i++ {
		wg.Add(1)
		go runPair(2000+i, 2001+i, msgs)
	}

	fmt.Printf("Waiting...\n")
	wg.Wait()
	// done!

	<-time.After(time.Microsecond * 100)
	if runtime.NumGoroutine() > 10 {
		// panic("uncomment me to debug")
		t.Fatal("leaking goroutines:", runtime.NumGoroutine())
	}
}
