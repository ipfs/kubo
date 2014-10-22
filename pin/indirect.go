package pin

import (
	"errors"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	"github.com/jbenet/go-ipfs/blocks/set"
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
	irefcnt, err := d.Get(k)
	if err != nil {
		return nil, err
	}
	refcnt, ok := irefcnt.(map[util.Key]int)
	if !ok {
		return nil, errors.New("invalid type from datastore")
	}

	var keys []util.Key
	for k, _ := range refcnt {
		keys = append(keys, k)
	}

	return &indirectPin{blockset: set.SimpleSetFromKeys(keys), refCounts: refcnt}, nil
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
