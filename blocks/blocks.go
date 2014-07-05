package blocks

import (
	"fmt"
	ds "github.com/jbenet/datastore.go"
	u "github.com/jbenet/go-ipfs/util"
	mh "github.com/jbenet/go-multihash"
)

// Blocks is the ipfs blocks service. It is the way
// to retrieve blocks by the higher level ipfs modules

type Block struct {
	Multihash mh.Multihash
	Data      []byte
}

func NewBlock(data []byte) (*Block, error) {
	h, err := u.Hash(data)
	if err != nil {
		return nil, err
	}
	return &Block{Data: data, Multihash: h}, nil
}

func (b *Block) Key() u.Key {
	return u.Key(b.Multihash)
}

type BlockService struct {
	Datastore ds.Datastore
	// Remote *bitswap.BitSwap // eventually.
}

func NewBlockService(d ds.Datastore) (*BlockService, error) {
	if d == nil {
		return nil, fmt.Errorf("BlockService requires valid datastore")
	}
	return &BlockService{Datastore: d}, nil
}

func (s *BlockService) AddBlock(b *Block) error {
	dsk := ds.NewKey(string(b.Key()))
	return s.Datastore.Put(dsk, b.Data)
}

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
