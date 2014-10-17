package pin

import (
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	bc "github.com/jbenet/go-ipfs/blocks/set"
	"github.com/jbenet/go-ipfs/util"
)

type indirectPin struct {
	blockset  bc.BlockSet
	refCounts map[util.Key]int
}

func loadBlockSet(d ds.Datastore) (bc.BlockSet, map[util.Key]int) {
	panic("Not yet implemented!")
	return nil, nil
}

func newIndirectPin(d ds.Datastore) indirectPin {
	// suppose the blockset actually takes blocks, not just keys
	bs, rc := loadBlockSet(d)
	return indirectPin{bs, rc}
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
