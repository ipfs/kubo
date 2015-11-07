package manet

import (
	"fmt"
	"net"
	"strings"

	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	utp "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net/utp"
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

	case "utp", "utp4", "utp6":
		acc, ok := a.(*utp.Addr)
		if !ok {
			return nil, errIncorrectNetAddr
		}

		// Get UDP Addr
		ac, ok := acc.Child().(*net.UDPAddr)
		if !ok {
			return nil, errIncorrectNetAddr
		}

		// Get IP Addr
		ipm, err := FromIP(ac.IP)
		if err != nil {
			return nil, errIncorrectNetAddr
		}

		// Get UDP Addr
		utpm, err := ma.NewMultiaddr(fmt.Sprintf("/udp/%d/utp", ac.Port))
		if err != nil {
			return nil, errIncorrectNetAddr
		}

		// Encapsulate
		return ipm.Encapsulate(utpm), nil

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
	case "tcp", "tcp4", "tcp6":
		return net.ResolveTCPAddr(network, host)
	case "udp", "udp4", "udp6":
		return net.ResolveUDPAddr(network, host)
	case "utp", "utp4", "utp6":
		return utp.ResolveAddr(network, host)
	case "ip", "ip4", "ip6":
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
	if parts[2] == "udp" && len(parts) > 4 && parts[4] == "utp" {
		network = parts[4]
	}

	var host string
	switch parts[0] {
	case "ip4":
		network = network + "4"
		host = strings.Join([]string{parts[1], parts[3]}, ":")
	case "ip6":
		network = network + "6"
		host = fmt.Sprintf("[%s]:%s", parts[1], parts[3])
	}
	return network, host, nil
}
