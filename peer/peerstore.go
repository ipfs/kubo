package peer

import (
	"errors"
	"sync"

	u "github.com/jbenet/go-ipfs/util"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
)

// Peerstore provides a threadsafe collection for peers.
type Peerstore interface {
	Get(ID) (*Peer, error)
	Add(*Peer) error
	Remove(ID) error
	All() (*map[u.Key]*Peer, error)
}

type peerstore struct {
	sync.RWMutex
	peers ds.Datastore
}

// NewPeerstore creates a threadsafe collection of peers.
func NewPeerstore() Peerstore {
	return &peerstore{
		peers: ds.NewMapDatastore(),
	}
}

func (p *peerstore) Get(i ID) (*Peer, error) {
	p.RLock()
	defer p.RUnlock()

	val, err := p.peers.Get(ds.NewKey(string(i)))
	if err != nil {
		return nil, err
	}

	peer, ok := val.(*Peer)
	if !ok {
		return nil, errors.New("stored value was not a Peer")
	}
	return peer, nil
}

func (p *peerstore) Add(peer *Peer) error {
	p.Lock()
	defer p.Unlock()

	k := ds.NewKey(string(peer.ID))
	return p.peers.Put(k, peer)
}

func (p *peerstore) Remove(i ID) error {
	p.Lock()
	defer p.Unlock()

	k := ds.NewKey(string(i))
	return p.peers.Delete(k)
}

func (p *peerstore) All() (*map[u.Key]*Peer, error) {
	p.RLock()
	defer p.RUnlock()

	l, err := p.peers.KeyList()
	if err != nil {
		return nil, err
	}

	ps := &map[u.Key]*Peer{}
	for _, k := range l {
		val, err := p.peers.Get(k)
		if err != nil {
			continue
		}

		pval, ok := val.(*Peer)
		if ok {
			(*ps)[u.Key(k.String())] = pval
		}
	}
	return ps, nil
}
