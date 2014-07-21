package blocks

import (
	"fmt"
	ds "github.com/jbenet/datastore.go"
	u "github.com/jbenet/go-ipfs/util"
	mh "github.com/jbenet/go-multihash"
)

// Block is the ipfs blocks service. It is the way
// to retrieve blocks by the higher level ipfs modules
type Block struct {
	Multihash mh.Multihash
	Data      []byte
}

// NewBlock creates a Block object from opaque data. It will hash the data.
func NewBlock(data []byte) (*Block, error) {
	h, err := u.Hash(data)
	if err != nil {
		return nil, err
	}
	return &Block{Data: data, Multihash: h}, nil
}

// Key returns the block's Multihash as a Key value.
func (b *Block) Key() u.Key {
	return u.Key(b.Multihash)
}

// BlockService is a block datastore.
// It uses an internal `datastore.Datastore` instance to store values.
type BlockService struct {
	Datastore ds.Datastore
	// Remote *bitswap.BitSwap // eventually.
}

// NewBlockService creates a BlockService with given datastore instance.
func NewBlockService(d ds.Datastore) (*BlockService, error) {
	if d == nil {
		return nil, fmt.Errorf("BlockService requires valid datastore")
	}
	return &BlockService{Datastore: d}, nil
}

// AddBlock adds a particular block to the service, Putting it into the datastore.
func (s *BlockService) AddBlock(b *Block) (u.Key, error) {
	k := b.Key()
	dsk := ds.NewKey(string(k))
	return k, s.Datastore.Put(dsk, b.Data)
}

// GetBlock retrieves a particular block from the service,
// Getting it from the datastore using the key (hash).
func (s *BlockService) GetBlock(k u.Key) (*Block, error) {
	dsk := ds.NewKey(string(k))
	datai, err := s.Datastore.Get(dsk)
	if err != nil {
		return nil, err
	}

	data, ok := datai.([]byte)
	if !ok {
		return nil, fmt.Errorf("data associated with %s is not a []byte", k)
	}

	return &Block{
		Multihash: mh.Multihash(k),
		Data:      data,
	}, nil
}
