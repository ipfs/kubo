package net

import (
	"bytes"
	"net"
	"sync"
	"testing"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

func newMultiaddr(t *testing.T, m string) ma.Multiaddr {
	maddr, err := ma.NewMultiaddr(m)
	if err != nil {
		t.Fatalf("failed to construct multiaddr: %s", m)
	}
	return maddr
}

func TestDial(t *testing.T) {

	listener, err := net.Listen("tcp", "127.0.0.1:4321")
	if err != nil {
		t.Fatal("failed to listen")
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {

		cB, err := listener.Accept()
		if err != nil {
			t.Fatal("failed to accept")
		}

		// echo out
		buf := make([]byte, 1024)
		for {
			_, err := cB.Read(buf)
			if err != nil {
				break
			}
			cB.Write(buf)
		}

		wg.Done()
	}()

	maddr := newMultiaddr(t, "/ip4/127.0.0.1/tcp/4321")
	cA, err := Dial(maddr)
	if err != nil {
		t.Fatal("failed to dial")
	}

	buf := make([]byte, 1024)
	if _, err := cA.Write([]byte("beep boop")); err != nil {
		t.Fatal("failed to write:", err)
	}

	if _, err := cA.Read(buf); err != nil {
		t.Fatal("failed to read:", buf, err)
	}

	if !bytes.Equal(buf[:9], []byte("beep boop")) {
		t.Fatal("failed to echo:", buf)
	}

	maddr2 := cA.RemoteMultiaddr()
	if !maddr2.Equal(maddr) {
		t.Fatal("remote multiaddr not equal:", maddr, maddr2)
	}

	cA.Close()
	wg.Wait()
}

func TestListen(t *testing.T) {

	maddr := newMultiaddr(t, "/ip4/127.0.0.1/tcp/4322")
	listener, err := Listen(maddr)
	if err != nil {
		t.Fatal("failed to listen")
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {

		cB, err := listener.Accept()
		if err != nil {
			t.Fatal("failed to accept")
		}

		if !cB.LocalMultiaddr().Equal(maddr) {
			t.Fatal("local multiaddr not equal:", maddr, cB.LocalMultiaddr())
		}

		// echo out
		buf := make([]byte, 1024)
		for {
			_, err := cB.Read(buf)
			if err != nil {
				break
			}
			cB.Write(buf)
		}

		wg.Done()
	}()

	cA, err := net.Dial("tcp", "127.0.0.1:4322")
	if err != nil {
		t.Fatal("failed to dial")
	}

	buf := make([]byte, 1024)
	if _, err := cA.Write([]byte("beep boop")); err != nil {
		t.Fatal("failed to write:", err)
	}

	if _, err := cA.Read(buf); err != nil {
		t.Fatal("failed to read:", buf, err)
	}

	if !bytes.Equal(buf[:9], []byte("beep boop")) {
		t.Fatal("failed to echo:", buf)
	}

	maddr2, err := FromNetAddr(cA.RemoteAddr())
	if err != nil {
		t.Fatal("failed to convert", err)
	}
	if !maddr2.Equal(maddr) {
		t.Fatal("remote multiaddr not equal:", maddr, maddr2)
	}

	cA.Close()
	wg.Wait()
}

func TestListenAndDial(t *testing.T) {

	maddr := newMultiaddr(t, "/ip4/127.0.0.1/tcp/4323")
	listener, err := Listen(maddr)
	if err != nil {
		t.Fatal("failed to listen")
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {

		cB, err := listener.Accept()
		if err != nil {
			t.Fatal("failed to accept")
		}

		if !cB.LocalMultiaddr().Equal(maddr) {
			t.Fatal("local multiaddr not equal:", maddr, cB.LocalMultiaddr())
		}

		// echo out
		buf := make([]byte, 1024)
		for {
			_, err := cB.Read(buf)
			if err != nil {
				break
			}
			cB.Write(buf)
		}

		wg.Done()
	}()

	cA, err := Dial(newMultiaddr(t, "/ip4/127.0.0.1/tcp/4323"))
	if err != nil {
		t.Fatal("failed to dial")
	}

	buf := make([]byte, 1024)
	if _, err := cA.Write([]byte("beep boop")); err != nil {
		t.Fatal("failed to write:", err)
	}

	if _, err := cA.Read(buf); err != nil {
		t.Fatal("failed to read:", buf, err)
	}

	if !bytes.Equal(buf[:9], []byte("beep boop")) {
		t.Fatal("failed to echo:", buf)
	}

	maddr2 := cA.RemoteMultiaddr()
	if !maddr2.Equal(maddr) {
		t.Fatal("remote multiaddr not equal:", maddr, maddr2)
	}

	cA.Close()
	wg.Wait()
}
