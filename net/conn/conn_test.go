package conn

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

func TestClose(t *testing.T) {
	// t.Skip("Skipping in favor of another test")

	ctx, cancel := context.WithCancel(context.Background())
	c1, c2 := setupConn(t, ctx, "/ip4/127.0.0.1/tcp/5534", "/ip4/127.0.0.1/tcp/5545")

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
		t.Fatal("not done after cancel")
	}

	c2.Close()

	select {
	case <-c2.Closed():
	default:
		t.Fatal("not done after cancel")
	}

	cancel() // close the listener :P
}

func TestCancel(t *testing.T) {
	// t.Skip("Skipping in favor of another test")

	ctx, cancel := context.WithCancel(context.Background())
	c1, c2 := setupConn(t, ctx, "/ip4/127.0.0.1/tcp/5534", "/ip4/127.0.0.1/tcp/5545")

	select {
	case <-c1.Closed():
		t.Fatal("done before close")
	case <-c2.Closed():
		t.Fatal("done before close")
	default:
	}

	cancel()

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

func TestCloseLeak(t *testing.T) {
	// t.Skip("Skipping in favor of another test")

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
	if runtime.NumGoroutine() > 20 {
		// panic("uncomment me to debug")
		t.Fatal("leaking goroutines:", runtime.NumGoroutine())
	}
}
