package blockstore

import (
	"errors"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"

	blocks "github.com/jbenet/go-ipfs/blocks"
	u "github.com/jbenet/go-ipfs/util"
)

var ValueTypeMismatch = errors.New("The retrieved value is not a Block")

type Blockstore interface {
	Get(u.Key) (*blocks.Block, error)
	Put(blocks.Block) error
}

func NewBlockstore(d ds.Datastore) Blockstore {
	return &blockstore{
		datastore: d,
	}
}

type blockstore struct {
	datastore ds.Datastore
}

func (bs *blockstore) Get(k u.Key) (*blocks.Block, error) {
	maybeData, err := bs.datastore.Get(toDatastoreKey(k))
	if err != nil {
		return nil, err
	}
	bdata, ok := maybeData.([]byte)
	if !ok {
		return nil, ValueTypeMismatch
	}
	return blocks.NewBlock(bdata)
}

func (bs *blockstore) Put(block blocks.Block) error {
	return bs.datastore.Put(toDatastoreKey(block.Key()), block.Data)
}

func toDatastoreKey(k u.Key) ds.Key {
	return ds.NewKey(string(k))
}
