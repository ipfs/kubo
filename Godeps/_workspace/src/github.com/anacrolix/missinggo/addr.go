package missinggo

import (
	"net"
	"strconv"
	"strings"
)

type HostMaybePort struct {
	Host   string
	Port   int
	NoPort bool
}

func (me HostMaybePort) String() string {
	if me.NoPort {
		return me.Host
	}
	return net.JoinHostPort(me.Host, strconv.FormatInt(int64(me.Port), 10))
}

func SplitHostPort(hostport string) (ret HostMaybePort) {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		if strings.Contains(err.Error(), "missing port") {
			ret.Host = hostport
			ret.NoPort = true
			return
		}
		panic(err)
	}
	i64, err := strconv.ParseInt(port, 0, 0)
	ret.Host = host
	ret.Port = int(i64)
	if err != nil {
		ret.NoPort = true
	}
	return
}

// Extracts the port as an integer from an address string.
func AddrPort(addr net.Addr) int {
	switch raw := addr.(type) {
	case *net.UDPAddr:
		return raw.Port
	default:
		_, port, err := net.SplitHostPort(addr.String())
		if err != nil {
			panic(err)
		}
		i64, err := strconv.ParseInt(port, 0, 0)
		if err != nil {
			panic(err)
		}
		return int(i64)
	}
}

func AddrIP(addr net.Addr) net.IP {
	switch raw := addr.(type) {
	case *net.UDPAddr:
		return raw.IP
	case *net.TCPAddr:
		return raw.IP
	default:
		host, _, err := net.SplitHostPort(addr.String())
		if err != nil {
			panic(err)
		}
		return net.ParseIP(host)
	}
}
