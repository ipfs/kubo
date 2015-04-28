package utp

import (
	"bytes"
	"testing"
)

func TestReadWrite(t *testing.T) {
	addr, err := ResolveAddr("utp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	l, err := Listen("utp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	payload := []byte("abcdefgh")

	ch := make(chan int)
	go func() {
		c, err := l.Accept()
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()

		var buf [256]byte
		length, err := c.Read(buf[:])
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(payload, buf[:length]) {
			t.Errorf("expected payload of %v; got %v", payload, buf[:length])
		}

		ch <- 0
	}()

	c, err := DialUTP("utp", nil, l.Addr().(*Addr))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	_, err = c.Write(payload)
	if err != nil {
		t.Fatal(err)
	}

	<-ch
}

func TestClose(t *testing.T) {
	addr, err := ResolveAddr("utp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	l, err := Listen("utp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	go func() {
		c, err := l.Accept()
		if err != nil {
			t.Fatal(err)
		}
		c.Close()
	}()

	c, err := DialUTP("utp", nil, l.Addr().(*Addr))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	var b [128]byte
	_, err = c.Read(b[:])
	if err == nil {
		t.Fatal("Read should fail")
	}
}
