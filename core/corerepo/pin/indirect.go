package pin

import (
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"

	"github.com/jbenet/go-ipfs/struct/blocks/set"
	"github.com/jbenet/go-ipfs/util"
)

type indirectPin struct {
	blockset  set.BlockSet
	refCounts map[util.Key]int
}

func NewIndirectPin(dstore ds.Datastore) *indirectPin {
	return &indirectPin{
		blockset:  set.NewDBWrapperSet(dstore, set.NewSimpleBlockSet()),
		refCounts: make(map[util.Key]int),
	}
}

func loadIndirPin(d ds.Datastore, k ds.Key) (*indirectPin, error) {
	var rcStore map[string]int
	err := loadSet(d, k, &rcStore)
	if err != nil {
		return nil, err
	}

	refcnt := make(map[util.Key]int)
	var keys []util.Key
	for encK, v := range rcStore {
		k := util.B58KeyDecode(encK)
		keys = append(keys, k)
		refcnt[k] = v
	}
	// log.Debugf("indirPin keys: %#v", keys)

	return &indirectPin{blockset: set.SimpleSetFromKeys(keys), refCounts: refcnt}, nil
}

func storeIndirPin(d ds.Datastore, k ds.Key, p *indirectPin) error {

	rcStore := map[string]int{}
	for k, v := range p.refCounts {
		rcStore[util.B58KeyEncode(k)] = v
	}
	return storeSet(d, k, rcStore)
}

func (i *indirectPin) Increment(k util.Key) {
	c := i.refCounts[k]
	i.refCounts[k] = c + 1
	if c <= 0 {
		i.blockset.AddBlock(k)
	}
}

func (i *indirectPin) Decrement(k util.Key) {
	c := i.refCounts[k] - 1
	i.refCounts[k] = c
	if c <= 0 {
		i.blockset.RemoveBlock(k)
	}
}

func (i *indirectPin) HasKey(k util.Key) bool {
	return i.blockset.HasKey(k)
}

func (i *indirectPin) Set() set.BlockSet {
	return i.blockset
}
