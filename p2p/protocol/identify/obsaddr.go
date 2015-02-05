package identify

import (
	"sync"
	"time"

	peer "github.com/jbenet/go-ipfs/p2p/peer"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

// ObservedAddr is an entry for an address reported by our peers.
// We only use addresses that:
// - have been observed more than once. (counter symmetric nats)
// - have been observed recently (10min), because our position in the
//   network, or network port mapppings, may have changed.
type ObservedAddr struct {
	Addr      ma.Multiaddr
	LastSeen  time.Time
	TimesSeen int
}

// ObservedAddrSet keeps track of a set of ObservedAddrs
// the zero-value is ready to be used.
type ObservedAddrSet struct {
	sync.Mutex // guards whole datastruct.

	addrs map[string]ObservedAddr
	ttl   time.Duration
}

func (oas *ObservedAddrSet) Addrs() []ma.Multiaddr {
	oas.Lock()
	defer oas.Unlock()

	// for zero-value.
	if oas.addrs == nil {
		return nil
	}

	now := time.Now()
	addrs := make([]ma.Multiaddr, 0, len(oas.addrs))
	for s, a := range oas.addrs {
		// remove timed out addresses.
		if now.Sub(a.LastSeen) > oas.ttl {
			delete(oas.addrs, s)
			continue
		}

		// we only use an address if we've seen it more than once
		// because symmetric nats may cause all our peers to see
		// different port numbers and thus report always different
		// addresses (different ports) for us. These wouldn't be
		// very useful. We make the assumption that if we've
		// connected to two different peers, and they both have
		// reported seeing the same address, it is probably useful.
		if a.TimesSeen > 1 {
			addrs = append(addrs, a.Addr)
		}
	}
	return addrs
}

func (oas *ObservedAddrSet) Add(addr ma.Multiaddr) {
	oas.Lock()
	defer oas.Unlock()

	// for zero-value.
	if oas.addrs == nil {
		oas.addrs = make(map[string]ObservedAddr)
		oas.ttl = peer.OwnObservedAddrTTL
	}

	s := addr.String()
	oas.addrs[s] = ObservedAddr{
		Addr:      addr,
		TimesSeen: oas.addrs[s].TimesSeen + 1,
		LastSeen:  time.Now(),
	}
}

func (oas *ObservedAddrSet) SetTTL(ttl time.Duration) {
	oas.Lock()
	defer oas.Unlock()
	oas.ttl = ttl
}

func (oas *ObservedAddrSet) TTL() time.Duration {
	oas.Lock()
	defer oas.Unlock()
	// for zero-value.
	if oas.addrs == nil {
		oas.ttl = peer.OwnObservedAddrTTL
	}
	return oas.ttl
}
