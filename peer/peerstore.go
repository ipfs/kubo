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

	k := u.Key(i).DsKey()
	val, err := p.peers.Get(k)
	switch err {

	// some other datastore error
	default:
		return nil, err

	// not found, construct it ourselves, add it to datastore, and return.
	case ds.ErrNotFound:
		peer := &Peer{ID: i}
		if err := p.peers.Put(k, peer); err != nil {
			return nil, err
		}
		return peer, nil

	// no error, got it back fine
	case nil:
		peer, ok := val.(*Peer)
		if !ok {
			return nil, errors.New("stored value was not a Peer")
		}
		return peer, nil
	}
}

func (p *peerstore) Put(peer *Peer) error {
	p.Lock()
	defer p.Unlock()

	k := u.Key(peer.ID).DsKey()
	return p.peers.Put(k, peer)
}

func (p *peerstore) Delete(i ID) error {
	p.Lock()
	defer p.Unlock()

	k := u.Key(i).DsKey()
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
			(*ps)[u.Key(pval.ID)] = pval
		}
	}
	return ps, nil
}
