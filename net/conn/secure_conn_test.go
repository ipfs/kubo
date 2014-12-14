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

func setupSecureConn(t *testing.T, ctx context.Context, c Conn) (Conn, error) {
	c, ok := c.(*secureConn)
	if ok {
		return c, nil
	}

	// shouldn't happen, because dial + listen already return secure conns.
	s, err := newSecureConn(ctx, c, peer.NewPeerstore())
	if err != nil {
		return nil, err
	}
	return s, nil
}

func TestSecureClose(t *testing.T) {
	// t.Skip("Skipping in favor of another test")

	ctx := context.Background()
	c1, c2 := setupConn(t, ctx, "/ip4/127.0.0.1/tcp/6634", "/ip4/127.0.0.1/tcp/6645")

	c1, err1 := setupSecureConn(t, ctx, c1)
	c2, err2 := setupSecureConn(t, ctx, c2)
	if err1 != nil {
		t.Fatal(err1)
	}
	if err2 != nil {
		t.Fatal(err2)
	}

	testOneSendRecv(t, c1, c2)
	testOneSendRecv(t, c2, c1)

	c1.Close()

	testNotOneSendRecv(t, c1, c2)
	testNotOneSendRecv(t, c2, c1)

}

func TestSecureCancelHandshake(t *testing.T) {
	// t.Skip("Skipping in favor of another test")

	ctx := context.Background()
	c1, c2 := setupConn(t, ctx, "/ip4/127.0.0.1/tcp/6634", "/ip4/127.0.0.1/tcp/6645")

	done := make(chan struct{})
	go func() {
		_, err1 := setupSecureConn(t, ctx, c1)
		_, err2 := setupSecureConn(t, ctx, c2)
		if err1 == nil {
			t.Fatal(err1)
		}
		if err2 == nil {
			t.Fatal(err2)
		}
		done <- struct{}{}
	}()

	<-done
}

func TestSecureCloseLeak(t *testing.T) {
	// t.Skip("Skipping in favor of another test")

	if testing.Short() {
		t.SkipNow()
	}
	if os.Getenv("TRAVIS") == "true" {
		t.Skip("this doesn't work well on travis")
	}

	var wg sync.WaitGroup

	runPair := func(p1, p2, num int) {
		a1 := strconv.Itoa(p1)
		a2 := strconv.Itoa(p2)
		ctx, cancel := context.WithCancel(context.Background())
		c1, c2 := setupConn(t, ctx, "/ip4/127.0.0.1/tcp/"+a1, "/ip4/127.0.0.1/tcp/"+a2)

		c1, err1 := setupSecureConn(t, ctx, c1)
		c2, err2 := setupSecureConn(t, ctx, c2)
		if err1 != nil {
			t.Fatal(err1)
		}
		if err2 != nil {
			t.Fatal(err2)
		}

		for i := 0; i < num; i++ {
			b1 := []byte("beep")
			c1.WriteMsg(b1)
			b2, err := c2.ReadMsg()
			if err != nil {
				panic(err)
			}
			if !bytes.Equal(b1, b2) {
				panic("bytes not equal")
			}

			b2 = []byte("beep")
			c2.WriteMsg(b2)
			b1, err = c1.ReadMsg()
			if err != nil {
				panic(err)
			}
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

	<-time.After(time.Millisecond * 150)
	if runtime.NumGoroutine() > 20 {
		// panic("uncomment me to debug")
		t.Fatal("leaking goroutines:", runtime.NumGoroutine())
	}
}
