package pin

import (
	key "gx/ipfs/Qmce4Y4zg3sYr7xKM5UueS67vhNni6EeWgCRnb7MbLJMew/go-key"
)

type indirectPin struct {
	refCounts map[key.Key]uint64
}

func newIndirectPin() *indirectPin {
	return &indirectPin{
		refCounts: make(map[key.Key]uint64),
	}
}

func (i *indirectPin) Increment(k key.Key) {
	i.refCounts[k]++
}

func (i *indirectPin) Decrement(k key.Key) {
	if i.refCounts[k] == 0 {
		log.Warningf("pinning: bad call: asked to unpin nonexistent indirect key: %v", k)
		return
	}
	i.refCounts[k]--
	if i.refCounts[k] == 0 {
		delete(i.refCounts, k)
	}
}

func (i *indirectPin) HasKey(k key.Key) bool {
	_, found := i.refCounts[k]
	return found
}

func (i *indirectPin) GetRefs() map[key.Key]uint64 {
	return i.refCounts
}
