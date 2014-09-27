package peer

import (
	"errors"
	"sync"

	u "github.com/jbenet/go-ipfs/util"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
)

// ErrNotFound signals a peer wasn't found. this is here to avoid having to
// leak the ds abstraction to clients of Peerstore, just for the error.
var ErrNotFound = ds.ErrNotFound

// Peerstore provides a threadsafe collection for peers.
type Peerstore interface {
	Get(ID) (*Peer, error)
	Put(*Peer) error
	Delete(ID) error
	All() (*Map, error)
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

	k := ds.NewKey(string(i))
	val, err := p.peers.Get(k)
	if err != nil {
		return nil, err
	}

	peer, ok := val.(*Peer)
	if !ok {
		return nil, errors.New("stored value was not a Peer")
	}
	return peer, nil
}

func (p *peerstore) Put(peer *Peer) error {
	p.Lock()
	defer p.Unlock()

	k := ds.NewKey(string(peer.ID))
	return p.peers.Put(k, peer)
}

func (p *peerstore) Delete(i ID) error {
	p.Lock()
	defer p.Unlock()

	k := ds.NewKey(string(i))
	return p.peers.Delete(k)
}

func (p *peerstore) All() (*Map, error) {
	p.RLock()
	defer p.RUnlock()

	l, err := p.peers.KeyList()
	if err != nil {
		return nil, err
	}

	ps := &Map{}
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
