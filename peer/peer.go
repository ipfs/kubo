package peer

import (
	"sync"
	"time"

	b58 "github.com/jbenet/go-base58"
	ic "github.com/jbenet/go-ipfs/crypto"
	u "github.com/jbenet/go-ipfs/util"
	ma "github.com/jbenet/go-multiaddr"
	mh "github.com/jbenet/go-multihash"

	"bytes"
)

// ID is a byte slice representing the identity of a peer.
type ID mh.Multihash

// Utililty function for comparing two peer ID's
func (id ID) Equal(other ID) bool {
	return bytes.Equal(id, other)
}

func (id ID) Pretty() string {
	return b58.Encode(id)
}

// Map maps Key (string) : *Peer (slices are not comparable).
type Map map[u.Key]*Peer

// Peer represents the identity information of an IPFS Node, including
// ID, and relevant Addresses.
type Peer struct {
	ID        ID
	Addresses []*ma.Multiaddr

	PrivKey ic.PrivKey
	PubKey  ic.PubKey

	latency   time.Duration
	latenLock sync.RWMutex
}

// Key returns the ID as a Key (string) for maps.
func (p *Peer) Key() u.Key {
	return u.Key(p.ID)
}

// AddAddress adds the given Multiaddr address to Peer's addresses.
func (p *Peer) AddAddress(a *ma.Multiaddr) {
	p.Addresses = append(p.Addresses, a)
}

// NetAddress returns the first Multiaddr found for a given network.
func (p *Peer) NetAddress(n string) *ma.Multiaddr {
	for _, a := range p.Addresses {
		ps, err := a.Protocols()
		if err != nil {
			continue // invalid addr
		}

		for _, p := range ps {
			if p.Name == n {
				return a
			}
		}
	}
	return nil
}

func (p *Peer) GetLatency() (out time.Duration) {
	p.latenLock.RLock()
	out = p.latency
	p.latenLock.RUnlock()
	return
}

// TODO: Instead of just keeping a single number,
//		 keep a running average over the last hour or so
func (p *Peer) SetLatency(laten time.Duration) {
	p.latenLock.Lock()
	p.latency = laten
	p.latenLock.Unlock()
}
