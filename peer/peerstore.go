package peer

import (
	"errors"
	"sync"

	u "github.com/jbenet/go-ipfs/util"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
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
	peers ds.Datastore
}

// NewPeerstore creates a threadsafe collection of peers.
func NewPeerstore() Peerstore {
	return &peerstore{
		peers: ds.NewMapDatastore(),
	}
}

func (p *peerstore) Get(i ID) (Peer, error) {
	p.Lock()
	defer p.Unlock()

	if i == nil {
		panic("wat")
	}

	k := u.Key(i).DsKey()
	val, err := p.peers.Get(k)
	switch err {

	// some other datastore error
	default:
		return nil, err

	// not found, construct it ourselves, add it to datastore, and return.
	case ds.ErrNotFound:

		// TODO(brian) kinda dangerous, no? If ID is invalid and doesn't
		// correspond to an actual valid peer ID, this peerstore will return an
		// instantiated peer value, allowing the error to propagate. It might
		// be better to nip this at the bud by returning nil and making the
		// client manually add a Peer. To keep the peerstore in control, this
		// can even be a peerstore method that performs cursory validation.
		//
		// Potential bad case: Suppose values arrive from untrusted providers
		// in the DHT.

		peer := &peer{id: i}
		if err := p.peers.Put(k, peer); err != nil {
			return nil, err
		}
		return peer, nil

	// no error, got it back fine
	case nil:
		peer, ok := val.(*peer)
		if !ok {
			return nil, errors.New("stored value was not a Peer")
		}
		return peer, nil
	}
}

func (p *peerstore) Add(peer Peer) (Peer, error) {
	p.Lock()
	defer p.Unlock()

	k := peer.Key().DsKey()
	val, err := p.peers.Get(k)
	switch err {
	// some other datastore error
	default:
		return nil, err

	// not found? just add and return.
	case ds.ErrNotFound:
		err := p.peers.Put(k, peer)
		return peer, err

	// no error, already here.
	case nil:
		peer2, ok := val.(Peer)
		if !ok {
			return nil, errors.New("stored value was not a Peer")
		}

		if peer == peer2 {
			return peer, nil
		}

		// must do some merging.
		peer2.Update(peer)
		return peer2, nil
	}
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

		pval, ok := val.(*peer)
		if ok {
			(*ps)[pval.Key()] = pval
		}
	}
	return ps, nil
}
