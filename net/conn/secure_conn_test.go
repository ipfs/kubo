package conn

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	peer "github.com/jbenet/go-ipfs/peer"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

func setupSecureConn(t *testing.T, c Conn) Conn {
	c, ok := c.(*secureConn)
	if ok {
		return c
	}

	// shouldn't happen, because dial + listen already return secure conns.
	s, err := newSecureConn(c.Context(), c, peer.NewPeerstore())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSecureClose(t *testing.T) {
	// t.Skip("Skipping in favor of another test")

	ctx, cancel := context.WithCancel(context.Background())
	c1, c2 := setupConn(t, ctx, "/ip4/127.0.0.1/tcp/6634", "/ip4/127.0.0.1/tcp/6645")

	c1 = setupSecureConn(t, c1)
	c2 = setupSecureConn(t, c2)

	select {
	case <-c1.Closed():
		t.Fatal("done before close")
	case <-c2.Closed():
		t.Fatal("done before close")
	default:
	}

	c1.Close()

	select {
	case <-c1.Closed():
	default:
		t.Fatal("not done after close")
	}

	c2.Close()

	select {
	case <-c2.Closed():
	default:
		t.Fatal("not done after close")
	}

	cancel() // close the listener :P
}

func TestSecureCancel(t *testing.T) {
	// t.Skip("Skipping in favor of another test")

	ctx, cancel := context.WithCancel(context.Background())
	c1, c2 := setupConn(t, ctx, "/ip4/127.0.0.1/tcp/6634", "/ip4/127.0.0.1/tcp/6645")

	c1 = setupSecureConn(t, c1)
	c2 = setupSecureConn(t, c2)

	select {
	case <-c1.Closed():
		t.Fatal("done before close")
	case <-c2.Closed():
		t.Fatal("done before close")
	default:
	}

	c1.Close()
	c2.Close()
	cancel() // listener

	// wait to ensure other goroutines run and close things.
	<-time.After(time.Microsecond * 10)
	// test that cancel called Close.

	select {
	case <-c1.Closed():
	default:
		t.Fatal("not done after cancel")
	}

	select {
	case <-c2.Closed():
	default:
		t.Fatal("not done after cancel")
	}

}

func TestSecureCloseLeak(t *testing.T) {
	// t.Skip("Skipping in favor of another test")
	if os.Getenv("TRAVIS") == "true" {
		t.Skip("this doesn't work well on travis")
	}

	var wg sync.WaitGroup

	runPair := func(p1, p2, num int) {
		a1 := strconv.Itoa(p1)
		a2 := strconv.Itoa(p2)
		ctx, cancel := context.WithCancel(context.Background())
		c1, c2 := setupConn(t, ctx, "/ip4/127.0.0.1/tcp/"+a1, "/ip4/127.0.0.1/tcp/"+a2)

		c1 = setupSecureConn(t, c1)
		c2 = setupSecureConn(t, c2)

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

		c1.Close()
		c2.Close()
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
	if runtime.NumGoroutine() > 20 {
		// panic("uncomment me to debug")
		t.Fatal("leaking goroutines:", runtime.NumGoroutine())
	}
}
