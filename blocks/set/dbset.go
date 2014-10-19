package set

import (
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	"github.com/jbenet/go-ipfs/blocks/bloom"
	"github.com/jbenet/go-ipfs/util"
)

type datastoreBlockSet struct {
	dstore ds.Datastore
	bset   BlockSet
}

func NewDBWrapperSet(d ds.Datastore, bset BlockSet) BlockSet {
	return &datastoreBlockSet{
		dstore: d,
		bset:   bset,
	}
}

func (d *datastoreBlockSet) AddBlock(k util.Key) {
	err := d.dstore.Put(k.DsKey(), []byte{})
	if err != nil {
		log.Error("blockset put error: %s", err)
	}

	d.bset.AddBlock(k)
}

func (d *datastoreBlockSet) RemoveBlock(k util.Key) {
	d.bset.RemoveBlock(k)
	if !d.bset.HasKey(k) {
		d.dstore.Delete(k.DsKey())
	}
}

func (d *datastoreBlockSet) HasKey(k util.Key) bool {
	return d.bset.HasKey(k)
}

func (d *datastoreBlockSet) GetBloomFilter() bloom.Filter {
	return d.bset.GetBloomFilter()
}

func (d *datastoreBlockSet) GetKeys() []util.Key {
	return d.bset.GetKeys()
}
