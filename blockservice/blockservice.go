// package blockservice implements a BlockService interface that provides
// a single GetBlock/AddBlock interface that seamlessly retrieves data either
// locally or from a remote peer through the exchange.
package blockservice

import (
	"errors"
	"fmt"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"

	blocks "github.com/jbenet/go-ipfs/blocks"
	exchange "github.com/jbenet/go-ipfs/exchange"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("blockservice")
var ErrNotFound = errors.New("blockservice: key not found")

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
	log.Debugf("blockservice: storing [%s] in datastore", k)
	// TODO(brian): define a block datastore with a Put method which accepts a
	// block parameter

	// check if we have it before adding. this is an extra read, but large writes
	// are more expensive.
	// TODO(jbenet) cheaper has. https://github.com/jbenet/go-datastore/issues/6
	has, err := s.Datastore.Has(k.DsKey())
	if err != nil {
		return k, err
	}
	if has {
		log.Debugf("blockservice: storing [%s] in datastore (already stored)", k)
	} else {
		log.Debugf("blockservice: storing [%s] in datastore", k)
		err := s.Datastore.Put(k.DsKey(), b.Data)
		if err != nil {
			return k, err
		}
	}

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
		blk, err := s.Remote.Block(ctx, k)
		if err != nil {
			return nil, err
		}
		return blk, nil
	} else {
		log.Debug("Blockservice GetBlock: Not found.")
		return nil, ErrNotFound
	}
}

// DeleteBlock deletes a block in the blockservice from the datastore
func (s *BlockService) DeleteBlock(k u.Key) error {
	return s.Datastore.Delete(k.DsKey())
}
