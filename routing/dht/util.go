package dht

import (
	"sync"

	peer "github.com/jbenet/go-ipfs/peer"
)

// Pool size is the number of nodes used for group find/set RPC calls
var PoolSize = 6

// We put the 'K' in kademlia!
var KValue = 10

// Its in the paper, i swear
var AlphaValue = 3

// A counter for incrementing a variable across multiple threads
type counter struct {
	n   int
	mut sync.Mutex
}

func (c *counter) Increment() {
	c.mut.Lock()
	c.n++
	c.mut.Unlock()
}

func (c *counter) Decrement() {
	c.mut.Lock()
	c.n--
	c.mut.Unlock()
}

func (c *counter) Size() (s int) {
	c.mut.Lock()
	s = c.n
	c.mut.Unlock()
	return
}

// peerSet is a threadsafe set of peers
type peerSet struct {
	ps map[string]bool
	lk sync.RWMutex
}

func newPeerSet() *peerSet {
	ps := new(peerSet)
	ps.ps = make(map[string]bool)
	return ps
}

func (ps *peerSet) Add(p *peer.Peer) {
	ps.lk.Lock()
	ps.ps[string(p.ID)] = true
	ps.lk.Unlock()
}

func (ps *peerSet) Contains(p *peer.Peer) bool {
	ps.lk.RLock()
	_, ok := ps.ps[string(p.ID)]
	ps.lk.RUnlock()
	return ok
}

func (ps *peerSet) Size() int {
	ps.lk.RLock()
	defer ps.lk.RUnlock()
	return len(ps.ps)
}

func (ps *peerSet) AddIfSmallerThan(p *peer.Peer, maxsize int) bool {
	var success bool
	ps.lk.Lock()
	if _, ok := ps.ps[string(p.ID)]; !ok && len(ps.ps) < maxsize {
		success = true
		ps.ps[string(p.ID)] = true
	}
	ps.lk.Unlock()
	return success
}
