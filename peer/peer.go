package peer

import (
	ma "github.com/jbenet/go-multiaddr"
	mh "github.com/jbenet/go-multihash"
)

type Peer struct {
	Id        mh.Multihash
	Addresses []*ma.Multiaddr
}

func (p *Peer) AddAddress(a *ma.Multiaddr) {
	p.Addresses = append(p.Addresses, a)
}

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
