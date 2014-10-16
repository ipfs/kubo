package pin

import (
	"github.com/jbenet/go-ipfs/blocks/bloom"
	"github.com/jbenet/go-ipfs/blocks/set"
	"github.com/jbenet/go-ipfs/util"
)

type refCntBlockSet struct {
	blocks map[util.Key]int
}

func NewRefCountBlockSet() set.BlockSet {
	return &refCntBlockSet{blocks: make(map[util.Key]int)}
}

func (r *refCntBlockSet) AddBlock(k util.Key) {
	r.blocks[k]++
}

func (r *refCntBlockSet) RemoveBlock(k util.Key) {
	v, ok := r.blocks[k]
	if !ok {
		return
	}
	if v <= 1 {
		delete(r.blocks, k)
	} else {
		r.blocks[k] = v - 1
	}
}

func (r *refCntBlockSet) HasKey(k util.Key) bool {
	_, ok := r.blocks[k]
	return ok
}

func (r *refCntBlockSet) GetBloomFilter() bloom.Filter {
	return nil
}
