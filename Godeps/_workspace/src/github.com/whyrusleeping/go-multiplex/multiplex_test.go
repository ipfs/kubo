package multiplex

import (
	"fmt"
	"io"
	"net"
	"testing"

	rand "github.com/dustin/randbo"
)

func TestBasicStreams(t *testing.T) {
	a, b := net.Pipe()

	mpa := NewMultiplex(a, false)
	mpb := NewMultiplex(b, true)

	mes := []byte("Hello world")
	go func() {
		s, err := mpb.Accept()
		if err != nil {
			t.Fatal(err)
		}

		_, err = s.Write(mes)
		if err != nil {
			t.Fatal(err)
		}

		err = s.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()

	s := mpa.NewStream()

	buf := make([]byte, len(mes))
	n, err := s.Read(buf)
	if err != nil {
		t.Fatal(err)
	}

	if n != len(mes) {
		t.Fatal("read wrong amount")
	}

	if string(buf) != string(mes) {
		t.Fatal("got bad data")
	}

	s.Close()

	mpa.Close()
	mpb.Close()
}

func TestEcho(t *testing.T) {
	a, b := net.Pipe()

	mpa := NewMultiplex(a, false)
	mpb := NewMultiplex(b, true)

	mes := make([]byte, 40960)
	rand.New().Read(mes)
	go func() {
		s, err := mpb.Accept()
		if err != nil {
			t.Fatal(err)
		}

		defer s.Close()
		io.Copy(s, s)
	}()

	s := mpa.NewStream()

	_, err := s.Write(mes)
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, len(mes))
	n, err := io.ReadFull(s, buf)
	if err != nil {
		t.Fatal(err)
	}

	if n != len(mes) {
		t.Fatal("read wrong amount")
	}

	if err := arrComp(buf, mes); err != nil {
		t.Fatal(err)
	}
	s.Close()

	mpa.Close()
	mpb.Close()
}

func arrComp(a, b []byte) error {
	msg := ""
	if len(a) != len(b) {
		msg += fmt.Sprintf("arrays differ in length: %d %d\n", len(a), len(b))
	}

	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			msg += fmt.Sprintf("content differs at index %d [%d != %d]", i, a[i], b[i])
			return fmt.Errorf(msg)
		}
	}
	if len(msg) > 0 {
		return fmt.Errorf(msg)
	}
	return nil
}
