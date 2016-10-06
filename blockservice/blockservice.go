// package blockservice implements a BlockService interface that provides
// a single GetBlock/AddBlock interface that seamlessly retrieves data either
// locally or from a remote peer through the exchange.
package blockservice

import (
	"errors"
	"fmt"

	blocks "github.com/ipfs/go-ipfs/blocks"
	"github.com/ipfs/go-ipfs/blocks/blockstore"
	exchange "github.com/ipfs/go-ipfs/exchange"

	context "context"
	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	cid "gx/ipfs/QmakyCk6Vnn16WEKjbkxieZmM2YLTzkFWizbmGowoYPjro/go-cid"
)

var log = logging.Logger("blockservice")

var ErrNotFound = errors.New("blockservice: key not found")

// BlockService is a hybrid block datastore. It stores data in a local
// datastore and may retrieve data from a remote Exchange.
// It uses an internal `datastore.Datastore` instance to store values.
type BlockService interface {
	Blockstore() blockstore.Blockstore
	Exchange() exchange.Interface
	AddBlock(o blocks.Block) (*cid.Cid, error)
	AddBlocks(bs []blocks.Block) ([]*cid.Cid, error)
	GetBlock(ctx context.Context, c *cid.Cid) (blocks.Block, error)
	GetBlocks(ctx context.Context, ks []*cid.Cid) <-chan blocks.Block
	DeleteBlock(o blocks.Block) error
	Close() error
}

type blockService struct {
	blockstore blockstore.Blockstore
	exchange   exchange.Interface
}

// NewBlockService creates a BlockService with given datastore instance.
func New(bs blockstore.Blockstore, rem exchange.Interface) BlockService {
	if rem == nil {
		log.Warning("blockservice running in local (offline) mode.")
	}

	return &blockService{
		blockstore: bs,
		exchange:   rem,
	}
}

func (bs *blockService) Blockstore() blockstore.Blockstore {
	return bs.blockstore
}

func (bs *blockService) Exchange() exchange.Interface {
	return bs.exchange
}

// AddBlock adds a particular block to the service, Putting it into the datastore.
// TODO pass a context into this if the remote.HasBlock is going to remain here.
func (s *blockService) AddBlock(o blocks.Block) (*cid.Cid, error) {
	// TODO: while this is a great optimization, we should think about the
	// possibility of streaming writes directly to disk. If we can pass this object
	// all the way down to the datastore without having to 'buffer' its data,
	// we could implement a `WriteTo` method on it that could do a streaming write
	// of the content, saving us (probably) considerable memory.
	c := o.Cid()
	has, err := s.blockstore.Has(c)
	if err != nil {
		return nil, err
	}

	if has {
		return c, nil
	}

	err = s.blockstore.Put(o)
	if err != nil {
		return nil, err
	}

	if err := s.exchange.HasBlock(o); err != nil {
		return nil, errors.New("blockservice is closed")
	}

	return c, nil
}

func (s *blockService) AddBlocks(bs []blocks.Block) ([]*cid.Cid, error) {
	var toput []blocks.Block
	for _, b := range bs {
		has, err := s.blockstore.Has(b.Cid())
		if err != nil {
			return nil, err
		}

		if has {
			continue
		}

		toput = append(toput, b)
	}

	err := s.blockstore.PutMany(toput)
	if err != nil {
		return nil, err
	}

	var ks []*cid.Cid
	for _, o := range toput {
		if err := s.exchange.HasBlock(o); err != nil {
			return nil, fmt.Errorf("blockservice is closed (%s)", err)
		}

		ks = append(ks, o.Cid())
	}
	return ks, nil
}

// GetBlock retrieves a particular block from the service,
// Getting it from the datastore using the key (hash).
func (s *blockService) GetBlock(ctx context.Context, c *cid.Cid) (blocks.Block, error) {
	log.Debugf("BlockService GetBlock: '%s'", c)

	block, err := s.blockstore.Get(c)
	if err == nil {
		return block, nil
	}

	if err == blockstore.ErrNotFound && s.exchange != nil {
		// TODO be careful checking ErrNotFound. If the underlying
		// implementation changes, this will break.
		log.Debug("Blockservice: Searching bitswap")
		blk, err := s.exchange.GetBlock(ctx, c)
		if err != nil {
			if err == blockstore.ErrNotFound {
				return nil, ErrNotFound
			}
			return nil, err
		}
		return blk, nil
	}

	log.Debug("Blockservice GetBlock: Not found")
	if err == blockstore.ErrNotFound {
		return nil, ErrNotFound
	}

	return nil, err
}

// GetBlocks gets a list of blocks asynchronously and returns through
// the returned channel.
// NB: No guarantees are made about order.
func (s *blockService) GetBlocks(ctx context.Context, ks []*cid.Cid) <-chan blocks.Block {
	out := make(chan blocks.Block, 0)
	go func() {
		defer close(out)
		var misses []*cid.Cid
		for _, c := range ks {
			hit, err := s.blockstore.Get(c)
			if err != nil {
				misses = append(misses, c)
				continue
			}
			log.Debug("Blockservice: Got data in datastore")
			select {
			case out <- hit:
			case <-ctx.Done():
				return
			}
		}

		if len(misses) == 0 {
			return
		}

		rblocks, err := s.exchange.GetBlocks(ctx, misses)
		if err != nil {
			log.Debugf("Error with GetBlocks: %s", err)
			return
		}

		for b := range rblocks {
			select {
			case out <- b:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// DeleteBlock deletes a block in the blockservice from the datastore
func (s *blockService) DeleteBlock(o blocks.Block) error {
	return s.blockstore.DeleteBlock(o.Cid())
}

func (s *blockService) Close() error {
	log.Debug("blockservice is shutting down...")
	return s.exchange.Close()
}
