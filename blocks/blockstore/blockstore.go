// package blockstore implements a thin wrapper over a datastore, giving a
// clean interface for Getting and Putting block objects.
package blockstore

import (
	"errors"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"

	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	blocks "github.com/jbenet/go-ipfs/blocks"
	u "github.com/jbenet/go-ipfs/util"
)

var ValueTypeMismatch = errors.New("The retrieved value is not a Block")

type Blockstore interface {
	DeleteBlock(u.Key) error
	Has(u.Key) (bool, error)
	Get(u.Key) (*blocks.Block, error)
	Put(*blocks.Block) error
}

func NewBlockstore(d ds.ThreadSafeDatastore) Blockstore {
	return &blockstore{
		datastore: d,
	}
}

type blockstore struct {
	datastore ds.ThreadSafeDatastore
}

func (bs *blockstore) Get(k u.Key) (*blocks.Block, error) {
	maybeData, err := bs.datastore.Get(k.DsKey())
	if err != nil {
		return nil, err
	}
	bdata, ok := maybeData.([]byte)
	if !ok {
		return nil, ValueTypeMismatch
	}

	return blocks.NewBlockWithHash(bdata, mh.Multihash(k))
}

func (bs *blockstore) Put(block *blocks.Block) error {
	return bs.datastore.Put(block.Key().DsKey(), block.Data)
}

func (bs *blockstore) Has(k u.Key) (bool, error) {
	return bs.datastore.Has(k.DsKey())
}

func (s *blockstore) DeleteBlock(k u.Key) error {
	return s.datastore.Delete(k.DsKey())
}
