package multiaddr

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestStringToBytes(t *testing.T) {

	testString := func(s string, h string) {
		b1, err := hex.DecodeString(h)
		if err != nil {
			t.Error("failed to decode hex", h)
		}

		b2, err := stringToBytes(s)
		if err != nil {
			t.Error("failed to convert", s)
		}

		if !bytes.Equal(b1, b2) {
			t.Error("failed to convert", s, "to", b1, "got", b2)
		}
	}

	testString("/ip4/127.0.0.1/udp/1234", "047f0000011104d2")
}

func TestBytesToString(t *testing.T) {

	testString := func(s1 string, h string) {
		b, err := hex.DecodeString(h)
		if err != nil {
			t.Error("failed to decode hex", h)
		}

		s2, err := bytesToString(b)
		if err != nil {
			t.Error("failed to convert", b)
		}

		if s1 != s2 {
			t.Error("failed to convert", b, "to", s1, "got", s2)
		}
	}

	testString("/ip4/127.0.0.1/udp/1234", "047f0000011104d2")
}

func TestProtocols(t *testing.T) {
	m, err := NewMultiaddr("/ip4/127.0.0.1/udp/1234")
	if err != nil {
		t.Error("failed to construct", "/ip4/127.0.0.1/udp/1234")
	}

	ps, err := m.Protocols()
	if err != nil {
		t.Error("failed to get protocols", "/ip4/127.0.0.1/udp/1234")
	}

	if ps[0] != ProtocolWithName("ip4") {
		t.Error(ps[0], ProtocolWithName("ip4"))
		t.Error("failed to get ip4 protocol")
	}

	if ps[1] != ProtocolWithName("udp") {
		t.Error(ps[1], ProtocolWithName("udp"))
		t.Error("failed to get udp protocol")
	}

}

func TestEncapsulate(t *testing.T) {
	m, err := NewMultiaddr("/ip4/127.0.0.1/udp/1234")
	if err != nil {
		t.Error(err)
	}

	m2, err := NewMultiaddr("/udp/5678")
	if err != nil {
		t.Error(err)
	}

	b := m.Encapsulate(m2)
	if s, _ := b.String(); s != "/ip4/127.0.0.1/udp/1234/udp/5678" {
		t.Error("encapsulate /ip4/127.0.0.1/udp/1234/udp/5678 failed.", s)
	}

	m3, _ := NewMultiaddr("/udp/5678")
	c, err := b.Decapsulate(m3)
	if err != nil {
		t.Error("decapsulate /udp failed.", err)
	}

	if s, _ := c.String(); s != "/ip4/127.0.0.1/udp/1234" {
		t.Error("decapsulate /udp failed.", "/ip4/127.0.0.1/udp/1234", s)
	}

	m4, _ := NewMultiaddr("/ip4/127.0.0.1")
	d, err := c.Decapsulate(m4)
	if err != nil {
		t.Error("decapsulate /ip4 failed.", err)
	}

	if s, _ := d.String(); s != "" {
		t.Error("decapsulate /ip4 failed.", "/", s)
	}
}

func TestDialArgs(t *testing.T) {
	m, err := NewMultiaddr("/ip4/127.0.0.1/udp/1234")
	if err != nil {
		t.Fatal("failed to construct", "/ip4/127.0.0.1/udp/1234")
	}

	nw, host, err := m.DialArgs()
	if err != nil {
		t.Fatal("failed to get dial args", "/ip4/127.0.0.1/udp/1234", err)
	}

	if nw != "udp" {
		t.Error("failed to get udp network Dial Arg")
	}

	if host != "127.0.0.1:1234" {
		t.Error("failed to get host:port Dial Arg")
	}
}
