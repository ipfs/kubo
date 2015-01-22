package reuseport

import (
	"net"
)

func ResolveAddr(network, address string) (net.Addr, error) {
	switch network {
	default:
		return nil, net.UnknownNetworkError(network)
	case "ip", "ip4", "ip6":
		return net.ResolveIPAddr(network, address)
	case "tcp", "tcp4", "tcp6":
		return net.ResolveTCPAddr(network, address)
	case "udp", "udp4", "udp6":
		return net.ResolveUDPAddr(network, address)
	case "unix", "unixgram", "unixpacket":
		return net.ResolveUnixAddr(network, address)
	}
}

// conn is a struct that stores a raddr to get around:
//  * https://github.com/golang/go/issues/9661#issuecomment-71043147
//  * https://gist.github.com/jbenet/5c191d698fe9ec58c49d
type conn struct {
	net.Conn
	raddr net.Addr
}

func (c *conn) RemoteAddr() net.Addr {
	return c.raddr
}
