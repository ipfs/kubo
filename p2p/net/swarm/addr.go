package swarm

import (
	conn "github.com/jbenet/go-ipfs/p2p/net/conn"
	eventlog "github.com/jbenet/go-ipfs/util/eventlog"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
)

// SupportedTransportStrings is the list of supported transports for the swarm.
// These are strings of encapsulated multiaddr protocols. E.g.:
//   /ip4/tcp
var SupportedTransportStrings = []string{
	"/ip4/tcp",
	"/ip6/tcp",
	// "/ip4/udp/utp", disabled because the lib is broken
	// "/ip6/udp/utp", disabled because the lib is broken
	// "/ip4/udp/udt", disabled because the lib doesnt work on arm
	// "/ip6/udp/udt", disabled because the lib doesnt work on arm
}

// SupportedTransportProtocols is the list of supported transports for the swarm.
// These are []ma.Protocol lists. Populated at runtime from SupportedTransportStrings
var SupportedTransportProtocols = [][]ma.Protocol{}

func init() {
	// initialize SupportedTransportProtocols
	transports := make([][]ma.Protocol, len(SupportedTransportStrings))
	for _, s := range SupportedTransportStrings {
		t, err := ma.ProtocolsWithString(s)
		if err != nil {
			panic(err) // important to fix this in the codebase
		}
		transports = append(transports, t)
	}
	SupportedTransportProtocols = transports
}

// FilterAddrs is a filter that removes certain addresses
// from a list. the addresses removed are those known NOT
// to work with swarm. Namely, addresses with UTP.
func FilterAddrs(a []ma.Multiaddr) []ma.Multiaddr {
	b := make([]ma.Multiaddr, 0, len(a))
	for _, addr := range a {
		if AddrUsable(addr) {
			b = append(b, addr)
		}
	}
	return b
}

// AddrUsable returns whether the swarm can use this addr.
func AddrUsable(a ma.Multiaddr) bool {
	// test the address protocol list is in SupportedTransportProtocols

	matches := func(a, b []ma.Protocol) bool {
		if len(a) != len(b) {
			return false
		}

		for i := range a {
			if a[i].Code != b[i].Code {
				return false
			}
		}
		return true
	}

	transport := a.Protocols()
	for _, supported := range SupportedTransportProtocols {
		if matches(supported, transport) {
			return true
		}
	}

	return false
}

// ListenAddresses returns a list of addresses at which this swarm listens.
func (s *Swarm) ListenAddresses() []ma.Multiaddr {
	listeners := s.swarm.Listeners()
	addrs := make([]ma.Multiaddr, 0, len(listeners))
	for _, l := range listeners {
		if l2, ok := l.NetListener().(conn.Listener); ok {
			addrs = append(addrs, l2.Multiaddr())
		}
	}
	return addrs
}

// InterfaceListenAddresses returns a list of addresses at which this swarm
// listens. It expands "any interface" addresses (/ip4/0.0.0.0, /ip6/::) to
// use the known local interfaces.
func InterfaceListenAddresses(s *Swarm) ([]ma.Multiaddr, error) {
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
func checkNATWarning(s *Swarm, observed ma.Multiaddr, expected ma.Multiaddr) {
	if observed.Equal(expected) {
		return
	}

	listen, err := InterfaceListenAddresses(s)
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
