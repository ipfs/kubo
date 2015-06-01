package pin

import (
	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	key "github.com/ipfs/go-ipfs/blocks/key"
	"github.com/ipfs/go-ipfs/blocks/set"
)

type indirectPin struct {
	blockset  set.BlockSet
	refCounts map[key.Key]int
}

func NewIndirectPin(dstore ds.Datastore) *indirectPin {
	return &indirectPin{
		blockset:  set.NewDBWrapperSet(dstore, set.NewSimpleBlockSet()),
		refCounts: make(map[key.Key]int),
	}
}

func loadIndirPin(d ds.Datastore, k ds.Key) (*indirectPin, error) {
	var rcStore map[string]int
	err := loadSet(d, k, &rcStore)
	if err != nil {
		return nil, err
	}

	refcnt := make(map[key.Key]int)
	var keys []key.Key
	for encK, v := range rcStore {
		if v > 0 {
			k := key.B58KeyDecode(encK)
			keys = append(keys, k)
			refcnt[k] = v
		}
	}
	// log.Debugf("indirPin keys: %#v", keys)

	return &indirectPin{blockset: set.SimpleSetFromKeys(keys), refCounts: refcnt}, nil
}

func storeIndirPin(d ds.Datastore, k ds.Key, p *indirectPin) error {

	rcStore := map[string]int{}
	for k, v := range p.refCounts {
		rcStore[key.B58KeyEncode(k)] = v
	}
	return storeSet(d, k, rcStore)
}

func (i *indirectPin) Increment(k key.Key) {
	c := i.refCounts[k]
	i.refCounts[k] = c + 1
	if c <= 0 {
		i.blockset.AddBlock(k)
	}
}

func (i *indirectPin) Decrement(k key.Key) {
	c := i.refCounts[k] - 1
	i.refCounts[k] = c
	if c <= 0 {
		i.blockset.RemoveBlock(k)
		delete(i.refCounts, k)
	}
}

func (i *indirectPin) HasKey(k key.Key) bool {
	return i.blockset.HasKey(k)
}

func (i *indirectPin) Set() set.BlockSet {
	return i.blockset
}

func (i *indirectPin) GetRefs() map[key.Key]int {
	return i.refCounts
}
