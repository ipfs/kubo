package utp

import (
	"net"
	"testing"
)

func TestListenerAccept(t *testing.T) {
	addr, err := ResolveAddr("utp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	l, err := Listen("utp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	uaddr, err := net.ResolveUDPAddr("udp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	uc, err := net.DialUDP("udp", nil, uaddr)
	if err != nil {
		t.Fatal(err)
	}
	defer uc.Close()

	for i := 0; i < 1; i++ {
		p := &packet{header: header{typ: stSyn, ver: version, id: uint16(i)}}
		payload, err := p.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		go func() {
			uc.Write(payload)
		}()

		a, err := l.Accept()
		if err != nil {
			t.Fatal(err)
		}
		a.Close()
	}
}

func TestListenerClose(t *testing.T) {
	addr, err := ResolveAddr("utp", ":0")
	if err != nil {
		t.Fatal(err)
	}

	l, err := Listen("utp", addr)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 5; i++ {
		l.Close()
	}

	_, err = l.Accept()
	if err == nil {
		t.Fatal("Accept should fail")
	}
}
