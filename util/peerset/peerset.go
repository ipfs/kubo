package peerset

import (
	peer "github.com/jbenet/go-ipfs/peer"
	"sync"
)

// PeerSet is a threadsafe set of peers
type PeerSet struct {
	ps   map[string]bool
	lk   sync.RWMutex
	size int
}

func NewPeerSet() *PeerSet {
	ps := new(PeerSet)
	ps.ps = make(map[string]bool)
	ps.size = -1
	return ps
}

func NewLimitedPeerSet(size int) *PeerSet {
	ps := new(PeerSet)
	ps.ps = make(map[string]bool)
	ps.size = size
	return ps
}

func (ps *PeerSet) Add(p peer.Peer) {
	ps.lk.Lock()
	ps.ps[string(p.ID())] = true
	ps.lk.Unlock()
}

func (ps *PeerSet) Contains(p peer.Peer) bool {
	ps.lk.RLock()
	_, ok := ps.ps[string(p.ID())]
	ps.lk.RUnlock()
	return ok
}

func (ps *PeerSet) Size() int {
	ps.lk.RLock()
	defer ps.lk.RUnlock()
	return len(ps.ps)
}

// TryAdd Attempts to add the given peer into the set.
// This operation can fail for one of two reasons:
// 1) The given peer is already in the set
// 2) The number of peers in the set is equal to size
func (ps *PeerSet) TryAdd(p peer.Peer) bool {
	var success bool
	ps.lk.Lock()
	if _, ok := ps.ps[string(p.ID())]; !ok && (len(ps.ps) < ps.size || ps.size == -1) {
		success = true
		ps.ps[string(p.ID())] = true
	}
	ps.lk.Unlock()
	return success
}
