package blockservice

import (
	"fmt"

	ds "github.com/jbenet/datastore.go"
	bitswap "github.com/jbenet/go-ipfs/bitswap"
	blocks "github.com/jbenet/go-ipfs/blocks"
	u "github.com/jbenet/go-ipfs/util"

	mh "github.com/jbenet/go-multihash"
)

// BlockService is a block datastore.
// It uses an internal `datastore.Datastore` instance to store values.
type BlockService struct {
	Datastore ds.Datastore
	Remote    *bitswap.BitSwap
}

// NewBlockService creates a BlockService with given datastore instance.
func NewBlockService(d ds.Datastore) (*BlockService, error) {
	if d == nil {
		return nil, fmt.Errorf("BlockService requires valid datastore")
	}
	return &BlockService{Datastore: d}, nil
}

// AddBlock adds a particular block to the service, Putting it into the datastore.
func (s *BlockService) AddBlock(b *blocks.Block) (u.Key, error) {
	k := b.Key()
	dsk := ds.NewKey(string(k))
	return k, s.Datastore.Put(dsk, b.Data)
}

// GetBlock retrieves a particular block from the service,
// Getting it from the datastore using the key (hash).
func (s *BlockService) GetBlock(k u.Key) (*blocks.Block, error) {
	dsk := ds.NewKey(string(k))
	datai, err := s.Datastore.Get(dsk)
	if err != nil {
		return nil, err
	}

	data, ok := datai.([]byte)
	if !ok {
		return nil, fmt.Errorf("data associated with %s is not a []byte", k)
	}

	return &blocks.Block{
		Multihash: mh.Multihash(k),
		Data:      data,
	}, nil
}
