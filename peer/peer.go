package peer

import (
	u "github.com/jbenet/go-ipfs/util"
	ma "github.com/jbenet/go-multiaddr"
	mh "github.com/jbenet/go-multihash"
)

// PeerId is a byte slice representing the identity of a peer.
type PeerId mh.Multihash

// PeerBook maps Key (string) : *Peer (slices are not comparable).
type PeerBook map[u.Key]*Peer

// Peer represents the identity information of an IPFS Node, including
// a PeerId, and relevant Addresses.
type Peer struct {
	Id        PeerId
	Addresses []*ma.Multiaddr
}

// Key returns the PeerId as a Key (string) for maps.
func (p *Peer) Key() u.Key {
	return u.Key(p.Id)
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
