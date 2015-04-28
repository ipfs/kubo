package utp

import "net"

// Addr represents the address of a UTP end point.
type Addr struct {
	net.Addr
}

// Network returns the address's network name, "utp".
func (a Addr) Network() string { return "utp" }

// ResolveAddr parses addr as a UTP address of the form "host:port"
// or "[ipv6-host%zone]:port" and resolves a pair of domain name and
// port name on the network net, which must be "utp", "utp4" or
// "utp6".  A literal address or host name for IPv6 must be enclosed
// in square brackets, as in "[::1]:80", "[ipv6-host]:http" or
// "[ipv6-host%zone]:80".
func ResolveAddr(n, addr string) (*Addr, error) {
	udpnet, err := utp2udp(n)
	if err != nil {
		return nil, err
	}
	udp, err := net.ResolveUDPAddr(udpnet, addr)
	if err != nil {
		return nil, err
	}
	return &Addr{Addr: udp}, nil
}

func utp2udp(n string) (string, error) {
	switch n {
	case "utp":
		return "udp", nil
	case "utp4":
		return "udp4", nil
	case "utp6":
		return "udp6", nil
	default:
		return "", net.UnknownNetworkError(n)
	}
}
