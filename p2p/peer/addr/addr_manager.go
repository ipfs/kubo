// package addr provides useful address utilities for p2p
// applications. It buys into the multi-transport addressing
// scheme Multiaddr, and uses it to build its own p2p addressing.
// All Addrs must have an associated peer.ID.
package addr

import (
	"sync"
	"time"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	peer "github.com/jbenet/go-ipfs/p2p/peer"
)

type expiringAddr struct {
	Addr ma.Multiaddr
	TTL  time.Time
}

func (e *expiringAddr) ExpiredBy(t time.Time) bool {
	return t.After(e.TTL)
}

type addrSet map[string]expiringAddr

// Manager manages addresses.
// The zero-value is ready to be used.
type Manager struct {
	addrmu sync.Mutex // guards addrs
	addrs  map[peer.ID]addrSet
}

// ensures the Manager is initialized.
// So we can use the zero value.
func (mgr *Manager) init() {
	if mgr.addrs == nil {
		mgr.addrs = make(map[peer.ID]addrSet)
	}
}

// AddAddr calls AddAddrs(p, []ma.Multiaddr{addr}, ttl)
func (mgr *Manager) AddAddr(p peer.ID, addr ma.Multiaddr, ttl time.Duration) {
	mgr.AddAddrs(p, []ma.Multiaddr{addr}, ttl)
}

// AddAddrs gives Manager addresses to use, with a given ttl
// (time-to-live), after which the address is no longer valid.
// If the manager has a longer TTL, the operation is a no-op for that address
func (mgr *Manager) AddAddrs(p peer.ID, addrs []ma.Multiaddr, ttl time.Duration) {
	mgr.addrmu.Lock()
	defer mgr.addrmu.Unlock()

	// if ttl is zero, exit. nothing to do.
	if ttl <= 0 {
		return
	}

	// so zero value can be used
	mgr.init()

	amap, found := mgr.addrs[p]
	if !found {
		amap = make(addrSet)
		mgr.addrs[p] = amap
	}

	// only expand ttls
	exp := time.Now().Add(ttl)
	for _, addr := range addrs {
		addrstr := addr.String()
		a, found := amap[addrstr]
		if !found || exp.After(a.TTL) {
			amap[addrstr] = expiringAddr{Addr: addr, TTL: exp}
		}
	}
}

// SetAddr calls mgr.SetAddrs(p, addr, ttl)
func (mgr *Manager) SetAddr(p peer.ID, addr ma.Multiaddr, ttl time.Duration) {
	mgr.SetAddrs(p, []ma.Multiaddr{addr}, ttl)
}

// SetAddrs sets the ttl on addresses. This clears any TTL there previously.
// This is used when we receive the best estimate of the validity of an address.
func (mgr *Manager) SetAddrs(p peer.ID, addrs []ma.Multiaddr, ttl time.Duration) {
	mgr.addrmu.Lock()
	defer mgr.addrmu.Unlock()

	// so zero value can be used
	mgr.init()

	amap, found := mgr.addrs[p]
	if !found {
		amap = make(addrSet)
		mgr.addrs[p] = amap
	}

	exp := time.Now().Add(ttl)
	for _, addr := range addrs {
		// re-set all of them for new ttl.
		addrs := addr.String()

		if ttl > 0 {
			amap[addrs] = expiringAddr{Addr: addr, TTL: exp}
		} else {
			delete(amap, addrs)
		}
	}
}

// Addresses returns all known (and valid) addresses for a given peer.
func (mgr *Manager) Addrs(p peer.ID) []ma.Multiaddr {
	mgr.addrmu.Lock()
	defer mgr.addrmu.Unlock()

	// not initialized? nothing to give.
	if mgr.addrs == nil {
		return nil
	}

	maddrs, found := mgr.addrs[p]
	if !found {
		return nil
	}

	now := time.Now()
	good := make([]ma.Multiaddr, 0, len(maddrs))
	var expired []string
	for s, m := range maddrs {
		if m.ExpiredBy(now) {
			expired = append(expired, s)
		} else {
			good = append(good, m.Addr)
		}
	}

	// clean up the expired ones.
	for _, s := range expired {
		delete(maddrs, s)
	}
	return good
}

// ClearAddresses removes all previously stored addresses
func (mgr *Manager) ClearAddrs(p peer.ID) {
	mgr.addrmu.Lock()
	defer mgr.addrmu.Unlock()
	mgr.init()

	mgr.addrs[p] = make(addrSet) // clear what was there before
}
