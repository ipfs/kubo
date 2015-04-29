package utp

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"
)

func TestSharedConnRecvPacket(t *testing.T) {
	addr, err := ResolveAddr("utp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	c, err := getSharedBaseConn("utp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	uaddr, err := net.ResolveUDPAddr("udp", c.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}

	uc, err := net.DialUDP("udp", nil, uaddr)
	if err != nil {
		t.Fatal(err)
	}
	defer uc.Close()

	ch := make(chan *packet)
	c.Register(5, ch)

	for i := 0; i < 100; i++ {
		p := &packet{header: header{typ: stData, ver: version, id: 5}}
		payload, err := p.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		go func() {
			uc.Write(payload)
		}()
		<-ch
	}

	c.Unregister(5)
}

func TestSharedConnSendPacket(t *testing.T) {
	addr, err := ResolveAddr("utp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	c, err := getSharedBaseConn("utp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	uaddr, err := net.ResolveUDPAddr("udp", c.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}

	uc, err := net.DialUDP("udp", nil, uaddr)
	if err != nil {
		t.Fatal(err)
	}
	defer uc.Close()

	for i := 0; i < 100; i++ {
		addr, err := net.ResolveUDPAddr("udp", uc.LocalAddr().String())
		if err != nil {
			t.Fatal(err)
		}
		p := &packet{header: header{typ: stData, ver: version, id: 5}, addr: addr}
		payload, err := p.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		c.Send(p)

		var b [256]byte
		l, err := uc.Read(b[:])
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(b[:l], payload) {
			t.Errorf("expected packet of %v; got %v", payload, b[:l])
		}
	}
}

func TestSharedConnRecvSyn(t *testing.T) {
	addr, err := ResolveAddr("utp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	c, err := getSharedBaseConn("utp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	uaddr, err := net.ResolveUDPAddr("udp", c.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}

	uc, err := net.DialUDP("udp", nil, uaddr)
	if err != nil {
		t.Fatal(err)
	}
	defer uc.Close()

	for i := 0; i < 100; i++ {
		p := &packet{header: header{typ: stSyn, ver: version}}
		payload, err := p.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		go func() {
			uc.Write(payload)
		}()
		p, err = c.RecvSyn(time.Duration(0))
		if err != nil {
			t.Fatal(err)
		}
		if p == nil {
			t.Errorf("packet must not be nil")
		}
	}
}

func TestSharedConnRecvOutOfBound(t *testing.T) {
	addr, err := ResolveAddr("utp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	c, err := getSharedBaseConn("utp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	uaddr, err := net.ResolveUDPAddr("udp", c.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}

	uc, err := net.DialUDP("udp", nil, uaddr)
	if err != nil {
		t.Fatal(err)
	}
	defer uc.Close()

	for i := 0; i < 100; i++ {
		payload := []byte("Hello")
		go func() {
			uc.Write(payload)
		}()
		var b [256]byte
		l, _, err := c.ReadFrom(b[:])
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(payload, b[:l]) {
			t.Errorf("expected packet of %v; got %v", payload, b[:l])
		}
	}
}

func TestSharedConnSendOutOfBound(t *testing.T) {
	addr, err := ResolveAddr("utp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	c, err := getSharedBaseConn("utp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	uaddr, err := net.ResolveUDPAddr("udp", c.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}

	uc, err := net.DialUDP("udp", nil, uaddr)
	if err != nil {
		t.Fatal(err)
	}
	defer uc.Close()

	for i := 0; i < 100; i++ {
		addr, err := net.ResolveUDPAddr("udp", uc.LocalAddr().String())
		if err != nil {
			t.Fatal(err)
		}
		payload := []byte("Hello")
		_, err = c.WriteTo(payload, addr)
		if err != nil {
			t.Fatal(err)
		}

		var b [256]byte
		l, err := uc.Read(b[:])
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(payload, b[:l]) {
			t.Errorf("expected packet of %v; got %v", payload, b[:l])
		}
	}
}

func TestSharedConnReferenceCount(t *testing.T) {
	addr, err := ResolveAddr("utp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	c, err := getSharedBaseConn("utp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	var w sync.WaitGroup

	c.Register(-1, nil)

	for i := 0; i < 5; i++ {
		w.Add(1)
		go func(i int) {
			defer w.Done()
			c.Register(int32(i), make(chan *packet))
		}(i)
	}

	w.Wait()
	for i := 0; i < 5; i++ {
		w.Add(1)
		go func(i int) {
			defer w.Done()
			c.Unregister(int32(i))
		}(i)
	}

	w.Wait()
	c.Unregister(-1)
	c.Close()

	c = baseConnMap[addr.String()]
	if c != nil {
		t.Errorf("baseConn should be released", c.ref)
	}
}

func TestSharedConnClose(t *testing.T) {
	addr, err := ResolveAddr("utp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	c, err := getSharedBaseConn("utp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	for i := 0; i < 5; i++ {
		c.Close()
	}

	var b [256]byte
	_, _, err = c.ReadFrom(b[:])
	if err == nil {
		t.Fatal("ReadFrom should fail")
	}

	uaddr, err := net.ResolveUDPAddr("udp", c.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}

	uc, err := net.DialUDP("udp", nil, uaddr)
	if err != nil {
		t.Fatal(err)
	}
	defer uc.Close()

	payload := []byte("Hello")
	_, err = c.WriteTo(payload, uc.LocalAddr())
	if err == nil {
		t.Fatal("WriteTo should fail")
	}
}
