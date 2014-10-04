package blockservice

import (
	"fmt"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	logging "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/op/go-logging"

	blocks "github.com/jbenet/go-ipfs/blocks"
	exchange "github.com/jbenet/go-ipfs/exchange"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("blockservice", logging.ERROR)

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
		log.Warning("blockservice running in local (offline) mode.")
	}
	return &BlockService{Datastore: d, Remote: rem}, nil
}

// AddBlock adds a particular block to the service, Putting it into the datastore.
func (s *BlockService) AddBlock(b *blocks.Block) (u.Key, error) {
	k := b.Key()
	log.Debug("storing [%s] in datastore", k.Pretty())
	// TODO(brian): define a block datastore with a Put method which accepts a
	// block parameter
	err := s.Datastore.Put(k.DsKey(), b.Data)
	if err != nil {
		return k, err
	}
	if s.Remote != nil {
		ctx := context.TODO()
		err = s.Remote.HasBlock(ctx, *b)
	}
	return k, err
}

// GetBlock retrieves a particular block from the service,
// Getting it from the datastore using the key (hash).
func (s *BlockService) GetBlock(k u.Key) (*blocks.Block, error) {
	log.Debug("BlockService GetBlock: '%s'", k.Pretty())
	datai, err := s.Datastore.Get(k.DsKey())
	if err == nil {
		log.Debug("Blockservice: Got data in datastore.")
		bdata, ok := datai.([]byte)
		if !ok {
			return nil, fmt.Errorf("data associated with %s is not a []byte", k)
		}
		return &blocks.Block{
			Multihash: mh.Multihash(k),
			Data:      bdata,
		}, nil
	} else if err == ds.ErrNotFound && s.Remote != nil {
		log.Debug("Blockservice: Searching bitswap.")
		ctx, _ := context.WithTimeout(context.TODO(), 5*time.Second)
		blk, err := s.Remote.Block(ctx, k)
		if err != nil {
			return nil, err
		}
		return blk, nil
	} else {
		log.Debug("Blockservice GetBlock: Not found.")
		return nil, u.ErrNotFound
	}
}
