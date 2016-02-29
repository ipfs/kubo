package addrutil

import (
	"fmt"

	logging "gx/ipfs/Qmazh5oNUVsDZTs2g59rq8aYQqwpss8tcUWQzor5sCCEuH/go-log"

	ma "gx/ipfs/QmR3JkmZBKYXgNMNsNZawm914455Qof3PEopwuVSeXG7aV/go-multiaddr"
	manet "gx/ipfs/QmYtzQmUwPFGxjCXctJ8e6GXS8sYfoXy2pdeMbS5SFWqRi/go-multiaddr-net"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

var log = logging.Logger("github.com/ipfs/go-libp2p/p2p/net/swarm/addr")

// SupportedTransportStrings is the list of supported transports for the swarm.
// These are strings of encapsulated multiaddr protocols. E.g.:
//   /ip4/tcp
var SupportedTransportStrings = []string{
	"/ip4/tcp",
	"/ip6/tcp",
	"/ip4/udp/utp",
	"/ip6/udp/utp",
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

// FilterAddrs is a filter that removes certain addresses, according to filter.
// if filter returns true, the address is kept.
func FilterAddrs(a []ma.Multiaddr, filter func(ma.Multiaddr) bool) []ma.Multiaddr {
	b := make([]ma.Multiaddr, 0, len(a))
	for _, addr := range a {
		if filter(addr) {
			b = append(b, addr)
		}
	}
	return b
}

// FilterUsableAddrs removes certain addresses
// from a list. the addresses removed are those known NOT
// to work with our network. Namely, addresses with UTP.
func FilterUsableAddrs(a []ma.Multiaddr) []ma.Multiaddr {
	return FilterAddrs(a, func(m ma.Multiaddr) bool {
		return AddrUsable(m, false)
	})
}

// AddrOverNonLocalIP returns whether the addr uses a non-local ip link
func AddrOverNonLocalIP(a ma.Multiaddr) bool {
	split := ma.Split(a)
	if len(split) < 1 {
		return false
	}
	if manet.IsIP6LinkLocal(split[0]) {
		return false
	}
	return true
}

// AddrUsable returns whether our network can use this addr.
// We only use the transports in SupportedTransportStrings,
// and we do not link local addresses. Loopback is ok
// as we need to be able to connect to multiple ipfs nodes
// in the same machine.
func AddrUsable(a ma.Multiaddr, partial bool) bool {
	if a == nil {
		return false
	}

	if !AddrOverNonLocalIP(a) {
		return false
	}

	// test the address protocol list is in SupportedTransportProtocols
	matches := func(supported, test []ma.Protocol) bool {
		if len(test) > len(supported) {
			return false
		}

		// when partial, it's ok if test < supported.
		if !partial && len(supported) != len(test) {
			return false
		}

		for i := range test {
			if supported[i].Code != test[i].Code {
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

// ResolveUnspecifiedAddress expands an unspecified ip addresses (/ip4/0.0.0.0, /ip6/::) to
// use the known local interfaces. If ifaceAddr is nil, we request interface addresses
// from the network stack. (this is so you can provide a cached value if resolving many addrs)
func ResolveUnspecifiedAddress(resolve ma.Multiaddr, ifaceAddrs []ma.Multiaddr) ([]ma.Multiaddr, error) {
	// split address into its components
	split := ma.Split(resolve)

	// if first component (ip) is not unspecified, use it as is.
	if !manet.IsIPUnspecified(split[0]) {
		return []ma.Multiaddr{resolve}, nil
	}

	out := make([]ma.Multiaddr, 0, len(ifaceAddrs))
	for _, ia := range ifaceAddrs {
		// must match the first protocol to be resolve.
		if ia.Protocols()[0].Code != resolve.Protocols()[0].Code {
			continue
		}

		split[0] = ia
		joined := ma.Join(split...)
		out = append(out, joined)
		log.Debug("adding resolved addr:", resolve, joined, out)
	}
	if len(out) < 1 {
		return nil, fmt.Errorf("failed to resolve: %s", resolve)
	}
	return out, nil
}

// ResolveUnspecifiedAddresses expands unspecified ip addresses (/ip4/0.0.0.0, /ip6/::) to
// use the known local interfaces.
func ResolveUnspecifiedAddresses(unspecAddrs, ifaceAddrs []ma.Multiaddr) ([]ma.Multiaddr, error) {

	// todo optimize: only fetch these if we have a "any" addr.
	if len(ifaceAddrs) < 1 {
		var err error
		ifaceAddrs, err = InterfaceAddresses()
		if err != nil {
			return nil, err
		}
		// log.Debug("InterfaceAddresses:", ifaceAddrs)
	}

	var outputAddrs []ma.Multiaddr
	for _, a := range unspecAddrs {
		// unspecified?
		resolved, err := ResolveUnspecifiedAddress(a, ifaceAddrs)
		if err != nil {
			continue // optimistic. if we cant resolve anything, we'll know at the bottom.
		}
		// log.Debug("resolved:", a, resolved)
		outputAddrs = append(outputAddrs, resolved...)
	}

	if len(outputAddrs) < 1 {
		return nil, fmt.Errorf("failed to specify addrs: %s", unspecAddrs)
	}

	log.Event(context.TODO(), "interfaceListenAddresses", func() logging.Loggable {
		var addrs []string
		for _, addr := range outputAddrs {
			addrs = append(addrs, addr.String())
		}
		return logging.Metadata{"addresses": addrs}
	}())

	log.Debug("ResolveUnspecifiedAddresses:", unspecAddrs, ifaceAddrs, outputAddrs)
	return outputAddrs, nil
}

// InterfaceAddresses returns a list of addresses associated with local machine
// Note: we do not return link local addresses. IP loopback is ok, because we
// may be connecting to other nodes in the same machine.
func InterfaceAddresses() ([]ma.Multiaddr, error) {
	maddrs, err := manet.InterfaceMultiaddrs()
	if err != nil {
		return nil, err
	}
	log.Debug("InterfaceAddresses: from manet:", maddrs)

	var out []ma.Multiaddr
	for _, a := range maddrs {
		if !AddrUsable(a, true) { // partial
			// log.Debug("InterfaceAddresses: skipping unusable:", a)
			continue
		}

		out = append(out, a)
	}

	log.Debug("InterfaceAddresses: usable:", out)
	return out, nil
}

// AddrInList returns whether or not an address is part of a list.
// this is useful to check if NAT is happening (or other bugs?)
func AddrInList(addr ma.Multiaddr, list []ma.Multiaddr) bool {
	for _, addr2 := range list {
		if addr.Equal(addr2) {
			return true
		}
	}
	return false
}

// AddrIsShareableOnWAN returns whether the given address should be shareable on the
// wide area network (wide internet).
func AddrIsShareableOnWAN(addr ma.Multiaddr) bool {
	s := ma.Split(addr)
	if len(s) < 1 {
		return false
	}
	a := s[0]
	if manet.IsIPLoopback(a) || manet.IsIP6LinkLocal(a) || manet.IsIPUnspecified(a) {
		return false
	}
	return manet.IsThinWaist(a)
}

// WANShareableAddrs filters addresses based on whether they're shareable on WAN
func WANShareableAddrs(inp []ma.Multiaddr) []ma.Multiaddr {
	return FilterAddrs(inp, AddrIsShareableOnWAN)
}

// Subtract filters out all addrs in b from a
func Subtract(a, b []ma.Multiaddr) []ma.Multiaddr {
	return FilterAddrs(a, func(m ma.Multiaddr) bool {
		for _, bb := range b {
			if m.Equal(bb) {
				return false
			}
		}
		return true
	})
}

// CheckNATWarning checks if our observed addresses differ. if so,
// informs the user that certain things might not work yet
func CheckNATWarning(observed, expected ma.Multiaddr, listen []ma.Multiaddr) {
	if observed.Equal(expected) {
		return
	}

	if !AddrInList(observed, listen) { // probably a nat
		log.Warningf(natWarning, observed, listen)
	}
}

const natWarning = `Remote peer observed our address to be: %s
The local addresses are: %s
Thus, connection is going through NAT, and other connections may fail.

IPFS NAT traversal is still under development. Please bug us on github or irc to fix this.
Baby steps: http://jbenet.static.s3.amazonaws.com/271dfcf/baby-steps.gif
`
