package peerset

import (
	peer "github.com/jbenet/go-ipfs/peer"
	"sync"
)

// PeerSet is a threadsafe set of peers
type PeerSet struct {
	ps map[string]bool
	lk sync.RWMutex
}

func NewPeerSet() *PeerSet {
	ps := new(PeerSet)
	ps.ps = make(map[string]bool)
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

func (ps *PeerSet) AddIfSmallerThan(p peer.Peer, maxsize int) bool {
	var success bool
	ps.lk.Lock()
	if _, ok := ps.ps[string(p.ID())]; !ok && len(ps.ps) < maxsize {
		success = true
		ps.ps[string(p.ID())] = true
	}
	ps.lk.Unlock()
	return success
}
