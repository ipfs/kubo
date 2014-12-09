package swarm

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
	"github.com/jbenet/go-ipfs/util/eventlog"

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
			outputAddrs = append(outputAddrs, a)
			continue
		}

		// unspecified? add one address per interface.
		for _, ia := range ifaceAddrs {
			split[0] = ia
			joined := ma.Join(split...)
			outputAddrs = append(outputAddrs, joined)
		}
	}

	log.Event(context.TODO(), "interfaceListenAddresses", func() eventlog.Loggable {
		var addrs []string
		for _, addr := range outputAddrs {
			addrs = append(addrs, addr.String())
		}
		return eventlog.Metadata{"addresses": addrs}
	}())
	log.Debug("InterfaceListenAddresses:", outputAddrs)
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

// addrInList returns whether or not an address is part of a list.
// this is useful to check if NAT is happening (or other bugs?)
func addrInList(addr ma.Multiaddr, list []ma.Multiaddr) bool {
	for _, addr2 := range list {
		if addr.Equal(addr2) {
			return true
		}
	}
	return false
}

// checkNATWarning checks if our observed addresses differ. if so,
// informs the user that certain things might not work yet
func (s *Swarm) checkNATWarning(observed ma.Multiaddr) {
	listen, err := s.InterfaceListenAddresses()
	if err != nil {
		log.Errorf("Error retrieving swarm.InterfaceListenAddresses: %s", err)
		return
	}

	if !addrInList(observed, listen) { // probably a nat
		log.Warningf(natWarning, observed, listen)
	}
}

const natWarning = `Remote peer observed our address to be: %s
The local addresses are: %s
Thus, connection is going through NAT, and other connections may fail.

IPFS NAT traversal is still under development. Please bug us on github or irc to fix this.
Baby steps: http://jbenet.static.s3.amazonaws.com/271dfcf/baby-steps.gif
`
