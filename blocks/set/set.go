package set

import (
	"github.com/jbenet/go-ipfs/blocks/bloom"
	"github.com/jbenet/go-ipfs/util"
)

type BlockSet interface {
	AddBlock(util.Key)
	RemoveBlock(util.Key)
	HasKey(util.Key) bool
	GetBloomFilter() bloom.Filter
}

func NewSimpleBlockSet() BlockSet {
	return &simpleBlockSet{blocks: make(map[util.Key]struct{})}
}

type simpleBlockSet struct {
	blocks map[util.Key]struct{}
}

func (b *simpleBlockSet) AddBlock(k util.Key) {
	b.blocks[k] = struct{}{}
}

func (b *simpleBlockSet) RemoveBlock(k util.Key) {
	delete(b.blocks, k)
}

func (b *simpleBlockSet) HasKey(k util.Key) bool {
	_, has := b.blocks[k]
	return has
}

func (b *simpleBlockSet) GetBloomFilter() bloom.Filter {
	f := bloom.BasicFilter()
	for k, _ := range b.blocks {
		f.Add([]byte(k))
	}
	return f
}
