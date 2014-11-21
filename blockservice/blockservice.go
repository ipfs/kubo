// package blockservice implements a BlockService interface that provides
// a single GetBlock/AddBlock interface that seamlessly retrieves data either
// locally or from a remote peer through the exchange.
package blockservice

import (
	"errors"
	"fmt"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"

	blocks "github.com/jbenet/go-ipfs/blocks"
	"github.com/jbenet/go-ipfs/blocks/blockstore"
	exchange "github.com/jbenet/go-ipfs/exchange"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("blockservice")
var ErrNotFound = errors.New("blockservice: key not found")

// BlockService is a hybrid block datastore. It stores data in a local
// datastore and may retrieve data from a remote Exchange.
// It uses an internal `datastore.Datastore` instance to store values.
type BlockService struct {
	// TODO don't expose underlying impl details
	Blockstore blockstore.Blockstore
	Remote     exchange.Interface
}

// NewBlockService creates a BlockService with given datastore instance.
func New(bs blockstore.Blockstore, rem exchange.Interface) (*BlockService, error) {
	if bs == nil {
		return nil, fmt.Errorf("BlockService requires valid blockstore")
	}
	if rem == nil {
		log.Warning("blockservice running in local (offline) mode.")
	}
	return &BlockService{Blockstore: bs, Remote: rem}, nil
}

// AddBlock adds a particular block to the service, Putting it into the datastore.
// TODO pass a context into this if the remote.HasBlock is going to remain here.
func (s *BlockService) AddBlock(b *blocks.Block) (u.Key, error) {
	k := b.Key()
	log.Debugf("blockservice: storing [%s] in datastore", k)
	// TODO(brian): define a block datastore with a Put method which accepts a
	// block parameter

	// check if we have it before adding. this is an extra read, but large writes
	// are more expensive.
	// TODO(jbenet) cheaper has. https://github.com/jbenet/go-datastore/issues/6
	has, err := s.Blockstore.Has(k)
	if err != nil {
		return k, err
	}
	if has {
		log.Debugf("blockservice: storing [%s] in datastore (already stored)", k)
	} else {
		log.Debugf("blockservice: storing [%s] in datastore", k)
		err := s.Blockstore.Put(b)
		if err != nil {
			return k, err
		}
	}

	// TODO this operation rate-limits blockservice operations, we should
	// consider moving this to an sync process.
	if s.Remote != nil {
		ctx := context.TODO()
		err = s.Remote.HasBlock(ctx, *b)
	}
	return k, err
}

// GetBlock retrieves a particular block from the service,
// Getting it from the datastore using the key (hash).
func (s *BlockService) GetBlock(ctx context.Context, k u.Key) (*blocks.Block, error) {
	log.Debugf("BlockService GetBlock: '%s'", k)
	block, err := s.Blockstore.Get(k)
	if err == nil {
		return block, nil
		// TODO be careful checking ErrNotFound. If the underlying
		// implementation changes, this will break.
	} else if err == ds.ErrNotFound && s.Remote != nil {
		log.Debug("Blockservice: Searching bitswap.")
		blk, err := s.Remote.GetBlock(ctx, k)
		if err != nil {
			return nil, err
		}
		return blk, nil
	} else {
		log.Debug("Blockservice GetBlock: Not found.")
		return nil, ErrNotFound
	}
}

func (s *BlockService) GetBlocks(ctx context.Context, ks []u.Key) <-chan *blocks.Block {
	out := make(chan *blocks.Block, 32)
	go func() {
		var toFetch []u.Key
		for _, k := range ks {
			block, err := s.Blockstore.Get(k)
			if err != nil {
				toFetch = append(toFetch, k)
				continue
			}
			log.Debug("Blockservice: Got data in datastore.")
			out <- block
		}
	}()
	return out
}

// DeleteBlock deletes a block in the blockservice from the datastore
func (s *BlockService) DeleteBlock(k u.Key) error {
	return s.Blockstore.DeleteBlock(k)
}
