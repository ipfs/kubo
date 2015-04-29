package utp

import (
	"io"
	"reflect"
	"testing"
)

func TestPacketBinary(t *testing.T) {
	h := header{
		typ:  stFin,
		ver:  version,
		id:   100,
		t:    50000,
		diff: 10000,
		wnd:  65535,
		seq:  100,
		ack:  200,
	}

	e := []extension{
		extension{
			typ:     extSelectiveAck,
			payload: []byte{0, 1, 0, 1},
		},
		extension{
			typ:     extSelectiveAck,
			payload: []byte{100, 0, 200, 0},
		},
	}

	p := packet{
		header:  h,
		ext:     e,
		payload: []byte("abcdefg"),
	}

	b, err := p.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	p2 := packet{payload: make([]byte, 0, mss)}
	err = p2.UnmarshalBinary(b)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(p, p2) {
		t.Errorf("expected packet of %v; got %v", p, p2)
	}
}

func TestUnmarshalShortPacket(t *testing.T) {
	b := make([]byte, 18)
	p := packet{}
	err := p.UnmarshalBinary(b)

	if err == nil {
		t.Fatal("UnmarshalBinary should fail")
	} else if err != io.EOF {
		t.Fatal(err)
	}
}
