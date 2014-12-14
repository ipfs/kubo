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

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

func testOneSendRecv(t *testing.T, c1, c2 Conn) {
	m1 := []byte("hello")
	if err := c1.WriteMsg(m1); err != nil {
		t.Fatal(err)
	}
	m2, err := c2.ReadMsg()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(m1, m2) {
		t.Fatal("failed to send: %s %s", m1, m2)
	}
}

func testNotOneSendRecv(t *testing.T, c1, c2 Conn) {
	m1 := []byte("hello")
	if err := c1.WriteMsg(m1); err == nil {
		t.Fatal("write should have failed", err)
	}
	_, err := c2.ReadMsg()
	if err == nil {
		t.Fatal("read should have failed", err)
	}
}

func TestClose(t *testing.T) {
	// t.Skip("Skipping in favor of another test")

	ctx := context.Background()
	c1, c2 := setupConn(t, ctx, "/ip4/127.0.0.1/tcp/5534", "/ip4/127.0.0.1/tcp/5545")

	testOneSendRecv(t, c1, c2)
	testOneSendRecv(t, c2, c1)

	c1.Close()

	time.After(200 * time.Millisecond)
	testNotOneSendRecv(t, c1, c2)
	testNotOneSendRecv(t, c2, c1)

	c2.Close()

	time.After(20000 * time.Millisecond)
	testNotOneSendRecv(t, c1, c2)
	testNotOneSendRecv(t, c2, c1)
}

func TestCloseLeak(t *testing.T) {
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

		for i := 0; i < num; i++ {
			b1 := []byte(fmt.Sprintf("beep%d", i))
			c1.WriteMsg(b1)
			b2, err := c2.ReadMsg()
			if err != nil {
				panic(err)
			}
			if !bytes.Equal(b1, b2) {
				panic(fmt.Errorf("bytes not equal: %s != %s", b1, b2))
			}

			b2 = []byte(fmt.Sprintf("boop%d", i))
			c2.WriteMsg(b2)
			b1, err = c1.ReadMsg()
			if err != nil {
				panic(err)
			}
			if !bytes.Equal(b1, b2) {
				panic(fmt.Errorf("bytes not equal: %s != %s", b1, b2))
			}

			<-time.After(time.Microsecond * 5)
		}

		c1.Close()
		c2.Close()
		cancel() // close the listener
		wg.Done()
	}

	var cons = 1
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
