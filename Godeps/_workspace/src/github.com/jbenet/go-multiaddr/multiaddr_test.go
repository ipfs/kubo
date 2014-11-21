package multiaddr

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func newMultiaddr(t *testing.T, a string) Multiaddr {
	m, err := NewMultiaddr(a)
	if err != nil {
		t.Error(err)
	}
	return m
}

func TestEqual(t *testing.T) {
	m1 := newMultiaddr(t, "/ip4/127.0.0.1/udp/1234")
	m2 := newMultiaddr(t, "/ip4/127.0.0.1/tcp/1234")
	m3 := newMultiaddr(t, "/ip4/127.0.0.1/tcp/1234")
	m4 := newMultiaddr(t, "/ip4/127.0.0.1/tcp/1234/")

	if m1.Equal(m2) {
		t.Error("should not be equal")
	}

	if m2.Equal(m1) {
		t.Error("should not be equal")
	}

	if !m2.Equal(m3) {
		t.Error("should be equal")
	}

	if !m3.Equal(m2) {
		t.Error("should be equal")
	}

	if !m1.Equal(m1) {
		t.Error("should be equal")
	}

	if !m2.Equal(m4) {
		t.Error("should be equal")
	}

	if !m4.Equal(m3) {
		t.Error("should be equal")
	}
}

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
	testString("/ip4/127.0.0.1/tcp/4321", "047f0000010610e1")
	testString("/ip4/127.0.0.1/udp/1234/ip4/127.0.0.1/tcp/4321", "047f0000011104d2047f0000010610e1")
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
	testString("/ip4/127.0.0.1/tcp/4321", "047f0000010610e1")
	testString("/ip4/127.0.0.1/udp/1234/ip4/127.0.0.1/tcp/4321", "047f0000011104d2047f0000010610e1")
}

func TestBytesSplitAndJoin(t *testing.T) {

	testString := func(s string, res []string) {
		m, err := NewMultiaddr(s)
		if err != nil {
			t.Fatal("failed to convert", s, err)
		}

		split := Split(m)
		if len(split) != len(res) {
			t.Error("not enough split components", split)
			return
		}

		for i, a := range split {
			if a.String() != res[i] {
				t.Errorf("split component failed: %s != %s", a, res[i])
			}
		}

		joined := Join(split...)
		if !m.Equal(joined) {
			t.Errorf("joined components failed: %s != %s", m, joined)
		}

		// modifying underlying bytes is fine.
		m2 := m.(*multiaddr)
		for i := range m2.bytes {
			m2.bytes[i] = 0
		}

		for i, a := range split {
			if a.String() != res[i] {
				t.Errorf("split component failed: %s != %s", a, res[i])
			}
		}
	}

	testString("/ip4/1.2.3.4/udp/1234", []string{"/ip4/1.2.3.4", "/udp/1234"})
	testString("/ip4/1.2.3.4/tcp/1/ip4/2.3.4.5/udp/2",
		[]string{"/ip4/1.2.3.4", "/tcp/1", "/ip4/2.3.4.5", "/udp/2"})
	testString("/ip4/1.2.3.4/utp/ip4/2.3.4.5/udp/2/udt",
		[]string{"/ip4/1.2.3.4", "/utp", "/ip4/2.3.4.5", "/udp/2", "/udt"})
}

func TestProtocols(t *testing.T) {
	m, err := NewMultiaddr("/ip4/127.0.0.1/udp/1234")
	if err != nil {
		t.Error("failed to construct", "/ip4/127.0.0.1/udp/1234")
	}

	ps := m.Protocols()
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
	if s := b.String(); s != "/ip4/127.0.0.1/udp/1234/udp/5678" {
		t.Error("encapsulate /ip4/127.0.0.1/udp/1234/udp/5678 failed.", s)
	}

	m3, _ := NewMultiaddr("/udp/5678")
	c := b.Decapsulate(m3)
	if s := c.String(); s != "/ip4/127.0.0.1/udp/1234" {
		t.Error("decapsulate /udp failed.", "/ip4/127.0.0.1/udp/1234", s)
	}

	m4, _ := NewMultiaddr("/ip4/127.0.0.1")
	d := c.Decapsulate(m4)
	if s := d.String(); s != "" {
		t.Error("decapsulate /ip4 failed.", "/", s)
	}
}
