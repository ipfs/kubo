package blockservice

import (
	"fmt"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"

	blocks "github.com/jbenet/go-ipfs/blocks"
	exchange "github.com/jbenet/go-ipfs/exchange"
	u "github.com/jbenet/go-ipfs/util"
)

// BlockService is a block datastore.
// It uses an internal `datastore.Datastore` instance to store values.
type BlockService struct {
	Datastore ds.Datastore
	Remote    exchange.Interface
}

// NewBlockService creates a BlockService with given datastore instance.
func NewBlockService(d ds.Datastore, rem exchange.Interface) (*BlockService, error) {
	if d == nil {
		return nil, fmt.Errorf("BlockService requires valid datastore")
	}
	if rem == nil {
		u.PErr("Caution: blockservice running in local (offline) mode.\n")
	}
	return &BlockService{Datastore: d, Remote: rem}, nil
}

// AddBlock adds a particular block to the service, Putting it into the datastore.
func (s *BlockService) AddBlock(b *blocks.Block) (u.Key, error) {
	k := b.Key()
	dsk := ds.NewKey(string(k))
	u.DOut("storing [%s] in datastore\n", k.Pretty())
	// TODO(brian): define a block datastore with a Put method which accepts a
	// block parameter
	err := s.Datastore.Put(dsk, b.Data)
	if err != nil {
		return k, err
	}
	if s.Remote != nil {
		err = s.Remote.HasBlock(*b)
	}
	return k, err
}

// GetBlock retrieves a particular block from the service,
// Getting it from the datastore using the key (hash).
func (s *BlockService) GetBlock(k u.Key) (*blocks.Block, error) {
	u.DOut("BlockService GetBlock: '%s'\n", k.Pretty())
	dsk := ds.NewKey(string(k))
	datai, err := s.Datastore.Get(dsk)
	if err == nil {
		u.DOut("Blockservice: Got data in datastore.\n")
		bdata, ok := datai.([]byte)
		if !ok {
			return nil, fmt.Errorf("data associated with %s is not a []byte", k)
		}
		return &blocks.Block{
			Multihash: mh.Multihash(k),
			Data:      bdata,
		}, nil
	} else if err == ds.ErrNotFound && s.Remote != nil {
		u.DOut("Blockservice: Searching bitswap.\n")
		ctx, _ := context.WithTimeout(context.TODO(), 5*time.Second)
		blk, err := s.Remote.Block(ctx, k)
		if err != nil {
			return nil, err
		}
		return blk, nil
	} else {
		u.DOut("Blockservice GetBlock: Not found.\n")
		return nil, u.ErrNotFound
	}
}
