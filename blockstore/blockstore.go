package blockstore

import (
	"errors"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"

	blocks "github.com/jbenet/go-ipfs/blocks"
	u "github.com/jbenet/go-ipfs/util"
)

var ValueTypeMismatch = errors.New("The retrieved value is not a Block")

type Blockstore interface {
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
	return blocks.NewBlock(bdata), nil
}

func (bs *blockstore) Put(block *blocks.Block) error {
	return bs.datastore.Put(block.Key().DsKey(), block.Data)
}
