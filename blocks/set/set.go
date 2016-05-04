// package set contains various different types of 'BlockSet's
package set

import (
	"github.com/ipfs/go-ipfs/blocks/bloom"
	key "github.com/ipfs/go-ipfs/blocks/key"
	logging "gx/ipfs/QmaDNZ4QMdBdku1YZWBysufYyoQt1negQGNav6PLYarbY8/go-log"
)

var log = logging.Logger("blockset")

// BlockSet represents a mutable set of keyed blocks
type BlockSet interface {
	AddBlock(key.Key)
	RemoveBlock(key.Key)
	HasKey(key.Key) bool
	GetBloomFilter() bloom.Filter

	GetKeys() []key.Key
}

func SimpleSetFromKeys(keys []key.Key) BlockSet {
	sbs := &simpleBlockSet{blocks: make(map[key.Key]struct{})}
	for _, k := range keys {
		sbs.blocks[k] = struct{}{}
	}
	return sbs
}

func NewSimpleBlockSet() BlockSet {
	return &simpleBlockSet{blocks: make(map[key.Key]struct{})}
}

type simpleBlockSet struct {
	blocks map[key.Key]struct{}
}

func (b *simpleBlockSet) AddBlock(k key.Key) {
	b.blocks[k] = struct{}{}
}

func (b *simpleBlockSet) RemoveBlock(k key.Key) {
	delete(b.blocks, k)
}

func (b *simpleBlockSet) HasKey(k key.Key) bool {
	_, has := b.blocks[k]
	return has
}

func (b *simpleBlockSet) GetBloomFilter() bloom.Filter {
	f := bloom.BasicFilter()
	for k := range b.blocks {
		f.Add([]byte(k))
	}
	return f
}

func (b *simpleBlockSet) GetKeys() []key.Key {
	var out []key.Key
	for k := range b.blocks {
		out = append(out, k)
	}
	return out
}
