package multiaddr

import (
	"net"
	"testing"
)

type GenFunc func() (Multiaddr, error)

func testConvert(t *testing.T, s string, gen GenFunc) {
	m, err := gen()
	if err != nil {
		t.Fatal("failed to generate.")
	}

	if s2 := m.String(); err != nil || s2 != s {
		t.Fatal("failed to convert: " + s + " != " + s2)
	}
}

func TestFromIP4(t *testing.T) {
	testConvert(t, "/ip4/10.20.30.40", func() (Multiaddr, error) {
		return FromIP(net.ParseIP("10.20.30.40"))
	})
}

func TestFromIP6(t *testing.T) {
	testConvert(t, "/ip6/2001:4860:0:2001::68", func() (Multiaddr, error) {
		return FromIP(net.ParseIP("2001:4860:0:2001::68"))
	})
}

func TestFromTCP(t *testing.T) {
	testConvert(t, "/ip4/10.20.30.40/tcp/1234", func() (Multiaddr, error) {
		return FromNetAddr(&net.TCPAddr{
			IP:   net.ParseIP("10.20.30.40"),
			Port: 1234,
		})
	})
}

func TestFromUDP(t *testing.T) {
	testConvert(t, "/ip4/10.20.30.40/udp/1234", func() (Multiaddr, error) {
		return FromNetAddr(&net.UDPAddr{
			IP:   net.ParseIP("10.20.30.40"),
			Port: 1234,
		})
	})
}

func TestDialArgs(t *testing.T) {
	m, err := NewMultiaddr("/ip4/127.0.0.1/udp/1234")
	if err != nil {
		t.Fatal("failed to construct", "/ip4/127.0.0.1/udp/1234")
	}

	nw, host, err := DialArgs(m)
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
