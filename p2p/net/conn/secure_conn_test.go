package conn

import (
	"bytes"
	"runtime"
	"sync"
	"testing"
	"time"

	ic "github.com/ipfs/go-ipfs/p2p/crypto"
	travis "github.com/ipfs/go-ipfs/util/testutil/ci/travis"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

func upgradeToSecureConn(t *testing.T, ctx context.Context, sk ic.PrivKey, c Conn) (Conn, error) {
	if c, ok := c.(*secureConn); ok {
		return c, nil
	}

	// shouldn't happen, because dial + listen already return secure conns.
	s, err := newSecureConn(ctx, sk, c)
	if err != nil {
		return nil, err
	}

	// need to read + write, as that's what triggers the handshake.
	h := []byte("hello")
	if _, err := s.Write(h); err != nil {
		return nil, err
	}
	if _, err := s.Read(h); err != nil {
		return nil, err
	}
	return s, nil
}

func secureHandshake(t *testing.T, ctx context.Context, sk ic.PrivKey, c Conn, done chan error) {
	_, err := upgradeToSecureConn(t, ctx, sk, c)
	done <- err
}

func TestSecureSimple(t *testing.T) {
	// t.Skip("Skipping in favor of another test")

	numMsgs := 100
	if testing.Short() {
		numMsgs = 10
	}

	ctx := context.Background()
	c1, c2, p1, p2 := setupSingleConn(t, ctx)

	done := make(chan error)
	go secureHandshake(t, ctx, p1.PrivKey, c1, done)
	go secureHandshake(t, ctx, p2.PrivKey, c2, done)

	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < numMsgs; i++ {
		testOneSendRecv(t, c1, c2)
		testOneSendRecv(t, c2, c1)
	}

	c1.Close()
	c2.Close()
}

func TestSecureClose(t *testing.T) {
	// t.Skip("Skipping in favor of another test")

	ctx := context.Background()
	c1, c2, p1, p2 := setupSingleConn(t, ctx)

	done := make(chan error)
	go secureHandshake(t, ctx, p1.PrivKey, c1, done)
	go secureHandshake(t, ctx, p2.PrivKey, c2, done)

	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}

	testOneSendRecv(t, c1, c2)

	c1.Close()
	testNotOneSendRecv(t, c1, c2)

	c2.Close()
	testNotOneSendRecv(t, c1, c2)
	testNotOneSendRecv(t, c2, c1)

}

func TestSecureCancelHandshake(t *testing.T) {
	// t.Skip("Skipping in favor of another test")

	ctx, cancel := context.WithCancel(context.Background())
	c1, c2, p1, p2 := setupSingleConn(t, ctx)

	done := make(chan error)
	go secureHandshake(t, ctx, p1.PrivKey, c1, done)
	<-time.After(time.Millisecond)
	cancel() // cancel ctx
	go secureHandshake(t, ctx, p2.PrivKey, c2, done)

	for i := 0; i < 2; i++ {
		if err := <-done; err == nil {
			t.Error("cancel should've errored out")
		}
	}
}

func TestSecureHandshakeFailsWithWrongKeys(t *testing.T) {
	// t.Skip("Skipping in favor of another test")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c1, c2, p1, p2 := setupSingleConn(t, ctx)

	done := make(chan error)
	go secureHandshake(t, ctx, p2.PrivKey, c1, done)
	go secureHandshake(t, ctx, p1.PrivKey, c2, done)

	for i := 0; i < 2; i++ {
		if err := <-done; err == nil {
			t.Fatal("wrong keys should've errored out.")
		}
	}
}

func TestSecureCloseLeak(t *testing.T) {
	// t.Skip("Skipping in favor of another test")

	if testing.Short() {
		t.SkipNow()
	}
	if travis.IsRunning() {
		t.Skip("this doesn't work well on travis")
	}

	runPair := func(c1, c2 Conn, num int) {
		log.Debugf("runPair %d", num)

		for i := 0; i < num; i++ {
			log.Debugf("runPair iteration %d", i)
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
	}

	var cons = 5
	var msgs = 50
	log.Debugf("Running %d connections * %d msgs.\n", cons, msgs)

	var wg sync.WaitGroup
	for i := 0; i < cons; i++ {
		wg.Add(1)

		ctx, cancel := context.WithCancel(context.Background())
		c1, c2, _, _ := setupSecureConn(t, ctx)
		go func(c1, c2 Conn) {

			defer func() {
				c1.Close()
				c2.Close()
				cancel()
				wg.Done()
			}()

			runPair(c1, c2, msgs)
		}(c1, c2)
	}

	log.Debugf("Waiting...\n")
	wg.Wait()
	// done!

	<-time.After(time.Millisecond * 150)
	if runtime.NumGoroutine() > 20 {
		// panic("uncomment me to debug")
		t.Fatal("leaking goroutines:", runtime.NumGoroutine())
	}
}
