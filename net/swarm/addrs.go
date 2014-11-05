package swarm

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr/net"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

// ListenAddresses returns a list of addresses at which this swarm listens.
func (s *Swarm) ListenAddresses() []ma.Multiaddr {
	addrs := make([]ma.Multiaddr, len(s.listeners))
	for i, l := range s.listeners {
		addrs[i] = l.Multiaddr()
	}
	return addrs
}

// InterfaceListenAddresses returns a list of addresses at which this swarm
// listens. It expands "any interface" addresses (/ip4/0.0.0.0, /ip6/::) to
// use the known local interfaces.
func (s *Swarm) InterfaceListenAddresses() ([]ma.Multiaddr, error) {
	return resolveUnspecifiedAddresses(s.ListenAddresses())
}

// resolveUnspecifiedAddresses expands unspecified ip addresses (/ip4/0.0.0.0, /ip6/::) to
// use the known local interfaces.
func resolveUnspecifiedAddresses(unspecifiedAddrs []ma.Multiaddr) ([]ma.Multiaddr, error) {
	var outputAddrs []ma.Multiaddr

	// todo optimize: only fetch these if we have a "any" addr.
	ifaceAddrs, err := interfaceAddresses()
	if err != nil {
		return nil, err
	}

	for _, a := range unspecifiedAddrs {

		// split address into its components
		split := ma.Split(a)

		// if first component (ip) is not unspecified, use it as is.
		if !manet.IsIPUnspecified(split[0]) {
			outputAddrs = append(outputAddrs)
			continue
		}

		// unspecified? add one address per interface.
		for _, ia := range ifaceAddrs {
			split[0] = ia
			joined := ma.Join(split...)
			outputAddrs = append(outputAddrs, joined)
		}
	}

	log.Info("InterfaceListenAddresses:", outputAddrs)
	return outputAddrs, nil
}

// interfaceAddresses returns a list of addresses associated with local machine
func interfaceAddresses() ([]ma.Multiaddr, error) {
	maddrs, err := manet.InterfaceMultiaddrs()
	if err != nil {
		return nil, err
	}

	var nonLoopback []ma.Multiaddr
	for _, a := range maddrs {
		if !manet.IsIPLoopback(a) {
			nonLoopback = append(nonLoopback, a)
		}
	}

	return nonLoopback, nil
}
