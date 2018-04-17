package cidextra

import (
	"fmt"

	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

var MaxIdHashLen = 64

type Opts struct {
	cid.Prefix
	idHashThres int // 0 disables, otherwise 1 + limit
}

// SetIdHashLimit is self explanatory
func (o *Opts) SetIdHashLimit(l int) error {
	if l > MaxIdHashLen {
		return fmt.Errorf("identity hash limit of %d larger then maxium allowed limit of %d",
			l, MaxIdHashLen)
	}
	o.idHashThres = l + 1
	return nil
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
