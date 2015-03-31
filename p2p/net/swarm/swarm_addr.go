package swarm

import (
	conn "github.com/ipfs/go-ipfs/p2p/net/conn"
	addrutil "github.com/ipfs/go-ipfs/p2p/net/swarm/addr"

	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

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
func (s *Swarm) InterfaceListenAddresses() ([]ma.Multiaddr, error) {
	return addrutil.ResolveUnspecifiedAddresses(s.ListenAddresses(), nil)
}

// checkNATWarning checks if our observed addresses differ. if so,
// informs the user that certain things might not work yet
func checkNATWarning(s *Swarm, observed ma.Multiaddr, expected ma.Multiaddr) {
	listen, err := s.InterfaceListenAddresses()
	if err != nil {
		log.Debugf("Error retrieving swarm.InterfaceListenAddresses: %s", err)
		return
	}

	addrutil.CheckNATWarning(observed, expected, listen)
}
