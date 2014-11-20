package utp

import "net"

type UTPAddr struct {
	net.Addr
}

func (a UTPAddr) Network() string { return "utp" }

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

func ResolveUTPAddr(n, addr string) (*UTPAddr, error) {
	udpnet, err := utp2udp(n)
	if err != nil {
		return nil, err
	}
	udp, err := net.ResolveUDPAddr(udpnet, addr)
	if err != nil {
		return nil, err
	}
	return &UTPAddr{Addr: udp}, nil
}
