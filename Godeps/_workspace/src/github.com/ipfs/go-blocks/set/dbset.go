package set

import (
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-blocks/bloom"
	key "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-blocks/key"
	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
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

func (d *datastoreBlockSet) AddBlock(k key.Key) error {
	err := d.dstore.Put(k.DsKey(), []byte{})
	if err != nil {
		return err
	}

	d.bset.AddBlock(k)
	return nil
}

func (d *datastoreBlockSet) RemoveBlock(k key.Key) error {
	d.bset.RemoveBlock(k)
	if !d.bset.HasKey(k) {
		return d.dstore.Delete(k.DsKey())
	}
	return nil
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
