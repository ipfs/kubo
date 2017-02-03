// package set contains various different types of 'BlockSet's
package set

import (
	"github.com/ipfs/go-ipfs/blocks/bloom"
	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	cid "gx/ipfs/QmV5gPoRsjN1Gid3LMdNZTyfCtP2DsvqEbMAmz82RmmiGk/go-cid"
)

var log = logging.Logger("blockset")

// BlockSet represents a mutable set of keyed blocks
type BlockSet interface {
	AddBlock(*cid.Cid)
	RemoveBlock(*cid.Cid)
	HasKey(*cid.Cid) bool
	GetBloomFilter() bloom.Filter

	GetKeys() []*cid.Cid
}

func SimpleSetFromKeys(keys []*cid.Cid) BlockSet {
	sbs := &simpleBlockSet{blocks: cid.NewSet()}
	for _, k := range keys {
		sbs.AddBlock(k)
	}
	return sbs
}

func NewSimpleBlockSet() BlockSet {
	return &simpleBlockSet{blocks: cid.NewSet()}
}

type simpleBlockSet struct {
	blocks *cid.Set
}

func (b *simpleBlockSet) AddBlock(k *cid.Cid) {
	b.blocks.Add(k)
}

func (b *simpleBlockSet) RemoveBlock(k *cid.Cid) {
	b.blocks.Remove(k)
}

func (b *simpleBlockSet) HasKey(k *cid.Cid) bool {
	return b.blocks.Has(k)
}

func (b *simpleBlockSet) GetBloomFilter() bloom.Filter {
	f := bloom.BasicFilter()
	for _, k := range b.blocks.Keys() {
		f.Add(k.Bytes())
	}
	return f
}

func (b *simpleBlockSet) GetKeys() []*cid.Cid {
	return b.blocks.Keys()
}
