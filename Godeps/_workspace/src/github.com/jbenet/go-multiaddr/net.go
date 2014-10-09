package multiaddr

import (
	"fmt"
	"net"
	"strings"
)

var errIncorrectNetAddr = fmt.Errorf("incorrect network addr conversion")

// FromNetAddr converts a net.Addr type to a Multiaddr.
func FromNetAddr(a net.Addr) (Multiaddr, error) {
	switch a.Network() {
	case "tcp", "tcp4", "tcp6":
		ac, ok := a.(*net.TCPAddr)
		if !ok {
			return nil, errIncorrectNetAddr
		}

		// Get IP Addr
		ipm, err := FromIP(ac.IP)
		if err != nil {
			return nil, errIncorrectNetAddr
		}

		// Get TCP Addr
		tcpm, err := NewMultiaddr(fmt.Sprintf("/tcp/%d", ac.Port))
		if err != nil {
			return nil, errIncorrectNetAddr
		}

		// Encapsulate
		return ipm.Encapsulate(tcpm), nil

	case "udp", "upd4", "udp6":
		ac, ok := a.(*net.UDPAddr)
		if !ok {
			return nil, errIncorrectNetAddr
		}

		// Get IP Addr
		ipm, err := FromIP(ac.IP)
		if err != nil {
			return nil, errIncorrectNetAddr
		}

		// Get UDP Addr
		udpm, err := NewMultiaddr(fmt.Sprintf("/udp/%d", ac.Port))
		if err != nil {
			return nil, errIncorrectNetAddr
		}

		// Encapsulate
		return ipm.Encapsulate(udpm), nil

	case "ip", "ip4", "ip6":
		ac, ok := a.(*net.IPAddr)
		if !ok {
			return nil, errIncorrectNetAddr
		}
		return FromIP(ac.IP)

	default:
		return nil, fmt.Errorf("unknown network %v", a.Network())
	}
}

// FromIP converts a net.IP type to a Multiaddr.
func FromIP(ip net.IP) (Multiaddr, error) {
	switch {
	case ip.To4() != nil:
		return NewMultiaddr("/ip4/" + ip.String())
	case ip.To16() != nil:
		return NewMultiaddr("/ip6/" + ip.String())
	default:
		return nil, errIncorrectNetAddr
	}
}

// DialArgs is a convenience function returning arguments for use in net.Dial
func DialArgs(m Multiaddr) (string, string, error) {
	if !IsThinWaist(m) {
		return "", "", fmt.Errorf("%s is not a 'thin waist' address", m)
	}

	str := m.String()
	parts := strings.Split(str, "/")[1:]
	network := parts[2]

	var host string
	switch parts[0] {
	case "ip4":
		host = strings.Join([]string{parts[1], parts[3]}, ":")
	case "ip6":
		host = fmt.Sprintf("[%s]:%s", parts[1], parts[3])
	}
	return network, host, nil
}

// IsThinWaist returns whether a Multiaddr starts with "Thin Waist" Protocols.
// This means: /{IP4, IP6}/{TCP, UDP}
func IsThinWaist(m Multiaddr) bool {
	p := m.Protocols()
	if p[0].Code != P_IP4 && p[0].Code != P_IP6 {
		return false
	}

	if p[1].Code != P_TCP && p[1].Code != P_UDP {
		return false
	}

	return true
}
