package multiaddr

import (
	"net"
	"testing"
)

type GenFunc func() (*Multiaddr, error)

func testConvert(t *testing.T, s string, gen GenFunc) {
	m, err := gen()
	if err != nil {
		t.Fatal("failed to generate.")
	}

	if s2, _ := m.String(); err != nil || s2 != s {
		t.Fatal("failed to convert: " + s + " != " + s2)
	}
}

func TestFromIP4(t *testing.T) {
	testConvert(t, "/ip4/10.20.30.40", func() (*Multiaddr, error) {
		return FromIP(net.ParseIP("10.20.30.40"))
	})
}

func TestFromIP6(t *testing.T) {
	testConvert(t, "/ip6/2001:4860:0:2001::68", func() (*Multiaddr, error) {
		return FromIP(net.ParseIP("2001:4860:0:2001::68"))
	})
}

func TestFromTCP(t *testing.T) {
	testConvert(t, "/ip4/10.20.30.40/tcp/1234", func() (*Multiaddr, error) {
		return FromNetAddr(&net.TCPAddr{
			IP:   net.ParseIP("10.20.30.40"),
			Port: 1234,
		})
	})
}

func TestFromUDP(t *testing.T) {
	testConvert(t, "/ip4/10.20.30.40/udp/1234", func() (*Multiaddr, error) {
		return FromNetAddr(&net.UDPAddr{
			IP:   net.ParseIP("10.20.30.40"),
			Port: 1234,
		})
	})
}
