package cidextra

import (
	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

type Opts struct {
	cid.Prefix 
	idHashThres int // 0 disables, otherwise 1 + limit
}

// SetIdHashLimit is self explanatory
func (o *Opts) SetIdHashLimit(l int) *Opts {
	// FIXME: Check against hard limit
	o.idHashThres = l + 1
	return o
}


// Sum returns a newly constructed Cid
func (o Opts) Sum(data []byte) (*cid.Cid, error) {
	if len(data) < o.idHashThres {
		prefix := o.Prefix
		prefix.MhType = mh.ID
		prefix.MhLength = -1
		return prefix.Sum(data)
	} else {
		return o.Prefix.Sum(data)
	}
}
