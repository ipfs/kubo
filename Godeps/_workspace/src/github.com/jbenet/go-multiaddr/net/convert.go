package manet

import (
	"fmt"
	"net"
	"strings"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

var errIncorrectNetAddr = fmt.Errorf("incorrect network addr conversion")

// FromNetAddr converts a net.Addr type to a Multiaddr.
func FromNetAddr(a net.Addr) (ma.Multiaddr, error) {
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
		tcpm, err := ma.NewMultiaddr(fmt.Sprintf("/tcp/%d", ac.Port))
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
		udpm, err := ma.NewMultiaddr(fmt.Sprintf("/udp/%d", ac.Port))
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

	case "ip+net":
		ac, ok := a.(*net.IPNet)
		if !ok {
			return nil, errIncorrectNetAddr
		}
		return FromIP(ac.IP)

	default:
		return nil, fmt.Errorf("unknown network %v", a.Network())
	}
}

// ToNetAddr converts a Multiaddr to a net.Addr
// Must be ThinWaist. acceptable protocol stacks are:
// /ip{4,6}/{tcp, udp}
func ToNetAddr(maddr ma.Multiaddr) (net.Addr, error) {
	network, host, err := DialArgs(maddr)
	if err != nil {
		return nil, err
	}

	switch network {
	case "tcp":
		return net.ResolveTCPAddr(network, host)
	case "udp":
		return net.ResolveUDPAddr(network, host)
	case "ip":
		return net.ResolveIPAddr(network, host)
	}

	return nil, fmt.Errorf("network not supported: %s", network)
}

// FromIP converts a net.IP type to a Multiaddr.
func FromIP(ip net.IP) (ma.Multiaddr, error) {
	switch {
	case ip.To4() != nil:
		return ma.NewMultiaddr("/ip4/" + ip.String())
	case ip.To16() != nil:
		return ma.NewMultiaddr("/ip6/" + ip.String())
	default:
		return nil, errIncorrectNetAddr
	}
}

// DialArgs is a convenience function returning arguments for use in net.Dial
func DialArgs(m ma.Multiaddr) (string, string, error) {
	if !IsThinWaist(m) {
		return "", "", fmt.Errorf("%s is not a 'thin waist' address", m)
	}

	str := m.String()
	parts := strings.Split(str, "/")[1:]

	if len(parts) == 2 { // only IP
		return parts[0], parts[1], nil
	}

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
