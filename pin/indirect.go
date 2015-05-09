package pin

import (
	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	key "github.com/ipfs/go-ipfs/blocks/key"
)

type indirectPin struct {
	refCounts map[key.Key]uint64
}

func newIndirectPin() *indirectPin {
	return &indirectPin{
		refCounts: make(map[key.Key]uint64),
	}
}

func loadIndirPin(d ds.Datastore, k ds.Key) (*indirectPin, error) {
	var rcStore map[string]uint64
	err := loadSet(d, k, &rcStore)
	if err != nil {
		return nil, err
	}

	refcnt := make(map[key.Key]uint64)
	var keys []key.Key
	for encK, v := range rcStore {
		if v > 0 {
			k := key.B58KeyDecode(encK)
			keys = append(keys, k)
			refcnt[k] = v
		}
	}
	// log.Debugf("indirPin keys: %#v", keys)

	return &indirectPin{refCounts: refcnt}, nil
}

func storeIndirPin(d ds.Datastore, k ds.Key, p *indirectPin) error {

	rcStore := map[string]uint64{}
	for k, v := range p.refCounts {
		rcStore[key.B58KeyEncode(k)] = v
	}
	return storeSet(d, k, rcStore)
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
