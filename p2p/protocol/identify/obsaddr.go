package identify

import (
	"sync"
	"time"

	peer "github.com/ipfs/go-ipfs/p2p/peer"

	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

// ObservedAddr is an entry for an address reported by our peers.
// We only use addresses that:
// - have been observed more than once. (counter symmetric nats)
// - have been observed recently (10min), because our position in the
//   network, or network port mapppings, may have changed.
type ObservedAddr struct {
	Addr     ma.Multiaddr
	SeenBy   map[string]struct{}
	LastSeen time.Time
}

// ObservedAddrSet keeps track of a set of ObservedAddrs
// the zero-value is ready to be used.
type ObservedAddrSet struct {
	sync.Mutex // guards whole datastruct.

	addrs map[string]*ObservedAddr
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
		//
		// Note: make sure not to double count observers.
		if len(a.SeenBy) > 1 {
			addrs = append(addrs, a.Addr)
		}
	}
	return addrs
}

func (oas *ObservedAddrSet) Add(addr ma.Multiaddr, observer ma.Multiaddr) {
	oas.Lock()
	defer oas.Unlock()

	// for zero-value.
	if oas.addrs == nil {
		oas.addrs = make(map[string]*ObservedAddr)
		oas.ttl = peer.OwnObservedAddrTTL
	}

	s := addr.String()
	oa, found := oas.addrs[s]

	// first time seeing address.
	if !found {
		oa = &ObservedAddr{
			Addr:   addr,
			SeenBy: make(map[string]struct{}),
		}
		oas.addrs[s] = oa
	}

	// mark the observer
	oa.SeenBy[observerGroup(observer)] = struct{}{}
	oa.LastSeen = time.Now()
}

// observerGroup is a function that determines what part of
// a multiaddr counts as a different observer. for example,
// two ipfs nodes at the same IP/TCP transport would get
// the exact same NAT mapping; they would count as the
// same observer. This may protect against NATs who assign
// different ports to addresses at different IP hosts, but
// not TCP ports.
//
// Here, we use the root multiaddr address. This is mostly
// IP addresses. In practice, this is what we want.
func observerGroup(m ma.Multiaddr) string {
	return ma.Split(m)[0].String()
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
