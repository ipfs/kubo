package set

import (
	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	"github.com/ipfs/go-ipfs/blocks/bloom"
	key "github.com/ipfs/go-ipfs/blocks/key"
)

type datastoreBlockSet struct {
	dstore ds.Datastore
	bset   BlockSet
}

// NewDBWrapperSet returns a new blockset wrapping a given datastore
func NewDBWrapperSet(d ds.Datastore, bset BlockSet) BlockSet {
	return &datastoreBlockSet{
		dstore: d,
		bset:   bset,
	}
}

func (d *datastoreBlockSet) AddBlock(k key.Key) {
	err := d.dstore.Put(k.DsKey(), []byte{})
	if err != nil {
		log.Debugf("blockset put error: %s", err)
	}

	d.bset.AddBlock(k)
}

func (d *datastoreBlockSet) RemoveBlock(k key.Key) {
	d.bset.RemoveBlock(k)
	if !d.bset.HasKey(k) {
		d.dstore.Delete(k.DsKey())
	}
}

func (d *datastoreBlockSet) HasKey(k key.Key) bool {
	return d.bset.HasKey(k)
}

func (d *datastoreBlockSet) GetBloomFilter() bloom.Filter {
	return d.bset.GetBloomFilter()
}

func (d *datastoreBlockSet) GetKeys() []key.Key {
	return d.bset.GetKeys()
}
