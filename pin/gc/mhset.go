package gc

import (
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
)

type MultihashSet map[string]struct{}

func (m MultihashSet) Add(h mh.Multihash) {
	m[string(h)] = struct{}{}
}

func (m MultihashSet) Has(h mh.Multihash) bool {
	_, ok := m[string(h)]
	return ok
}

func MultihashSetFromCids(cids *cid.Set) MultihashSet {
	mhSet := make(MultihashSet, cids.Len())

	if err := cids.ForEach(func(c *cid.Cid) error {
		mhSet.Add(c.Hash())
		return nil
	}); err != nil {
		log.Panic(err)
	}
	return mhSet
}
