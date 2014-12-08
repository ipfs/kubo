package peer

import (
	"sync"

	u "github.com/jbenet/go-ipfs/util"
)

// Peerstore provides a threadsafe collection for peers.
type Peerstore interface {
	Get(ID) (Peer, error)
	Add(Peer) (Peer, error)
	Delete(ID) error
	All() (*Map, error)
}

type peerstore struct {
	sync.RWMutex
	data map[string]Peer // key is string(ID)
}

// NewPeerstore creates a threadsafe collection of peers.
func NewPeerstore() Peerstore {
	return &peerstore{
		data: make(map[string]Peer),
	}
}

func (ps *peerstore) Get(i ID) (Peer, error) {
	ps.Lock()
	defer ps.Unlock()

	if i == nil {
		panic("wat")
	}

	p, ok := ps.data[i.String()]
	if !ok { // not found, construct it ourselves, add it to datastore, and return.

		// TODO(brian) kinda dangerous, no? If ID is invalid and doesn't
		// correspond to an actual valid peer ID, this peerstore will return an
		// instantiated peer value, allowing the error to propagate. It might
		// be better to nip this at the bud by returning nil and making the
		// client manually add a Peer. To keep the peerstore in control, this
		// can even be a peerstore method that performs cursory validation.
		//
		// Potential bad case: Suppose values arrive from untrusted providers
		// in the DHT.
		p = &peer{id: i}
		ps.data[i.String()] = p
	}

	// no error, got it back fine
	return p, nil
}

func (p *peerstore) Add(peer Peer) (Peer, error) {
	p.Lock()
	defer p.Unlock()

	existing, ok := p.data[peer.Key().String()]
	if !ok { // not found? just add and return.
		p.data[peer.Key().String()] = peer
		return peer, nil
	}
	// already here.
	if peer == existing {
		return peer, nil
	}
	existing.Update(peer) // must do some merging.
	return existing, nil
}

func (p *peerstore) Delete(i ID) error {
	p.Lock()
	defer p.Unlock()

	delete(p.data, i.String())
	return nil
}

func (p *peerstore) All() (*Map, error) {
	p.Lock()
	defer p.Unlock()

	ps := Map{}
	for k, v := range p.data {
		ps[u.Key(k)] = v
	}
	return &ps, nil
}
