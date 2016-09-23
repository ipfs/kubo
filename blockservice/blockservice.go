// package blockservice implements a BlockService interface that provides
// a single GetBlock/AddBlock interface that seamlessly retrieves data either
// locally or from a remote peer through the exchange.
package blockservice

import (
	"errors"

	blocks "github.com/ipfs/go-ipfs/blocks"
	"github.com/ipfs/go-ipfs/blocks/blockstore"
	exchange "github.com/ipfs/go-ipfs/exchange"
	key "gx/ipfs/Qmce4Y4zg3sYr7xKM5UueS67vhNni6EeWgCRnb7MbLJMew/go-key"

	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	cid "gx/ipfs/QmfSc2xehWmWLnwwYR91Y8QF4xdASypTFVknutoKQS3GHp/go-cid"
)

var log = logging.Logger("blockservice")

var ErrNotFound = errors.New("blockservice: key not found")

// BlockService is a hybrid block datastore. It stores data in a local
// datastore and may retrieve data from a remote Exchange.
// It uses an internal `datastore.Datastore` instance to store values.
type BlockService struct {
	// TODO don't expose underlying impl details
	Blockstore blockstore.Blockstore
	Exchange   exchange.Interface
}

// an Object is simply a typed block
type Object interface {
	Cid() *cid.Cid
	blocks.Block
}

// NewBlockService creates a BlockService with given datastore instance.
func New(bs blockstore.Blockstore, rem exchange.Interface) *BlockService {
	if rem == nil {
		log.Warning("blockservice running in local (offline) mode.")
	}

	return &BlockService{
		Blockstore: bs,
		Exchange:   rem,
	}
}

// AddBlock adds a particular block to the service, Putting it into the datastore.
// TODO pass a context into this if the remote.HasBlock is going to remain here.
func (s *BlockService) AddObject(o Object) (*cid.Cid, error) {
	err, added := s.Blockstore.Put(o)
	if err != nil {
		return nil, err
	}
	if added == nil {
		return o.Cid(), nil
	}

	if err := s.Exchange.HasBlock(o); err != nil {
		return nil, errors.New("blockservice is closed")
	}

	return o.Cid(), nil
}

func (s *BlockService) AddObjects(bs []Object) ([]*cid.Cid, error) {
	cids := make([]*cid.Cid, 0, len(bs))
	blks := make([]blocks.Block, 0, len(bs))
	for _, b := range bs {
		cids = append(cids, b.Cid())
		blks = append(blks, b)
	}
	
	err, added := s.Blockstore.PutMany(blks)
	if err != nil {
		return nil, err
	}

	for _, o := range added {
		if err := s.Exchange.HasBlock(o); err != nil {
			return nil, errors.New("blockservice is closed")
		}
	}
	return cids, nil
}

// GetBlock retrieves a particular block from the service,
// Getting it from the datastore using the key (hash).
func (s *BlockService) GetBlock(ctx context.Context, c *cid.Cid) (blocks.Block, error) {
	log.Debugf("BlockService GetBlock: '%s'", c)
	
	block, err := s.Blockstore.Get(key.Key(c.Hash()))
	if err == nil {
		return block, nil
	}

	if err == blockstore.ErrNotFound && s.Exchange != nil {
		// TODO be careful checking ErrNotFound. If the underlying
		// implementation changes, this will break.
		log.Debug("Blockservice: Searching bitswap")
		blk, err := s.Exchange.GetBlock(ctx, key.Key(c.Hash()))
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
func (s *BlockService) GetBlocks(ctx context.Context, ks []*cid.Cid) <-chan blocks.Block {
	out := make(chan blocks.Block, 0)
	go func() {
		defer close(out)
		var misses []key.Key
		for _, c := range ks {
			k := key.Key(c.Hash())
			hit, err := s.Blockstore.Get(k)
			if err != nil {
				misses = append(misses, k)
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

		rblocks, err := s.Exchange.GetBlocks(ctx, misses)
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
func (s *BlockService) DeleteObject(o Object) error {
	return s.Blockstore.DeleteBlock(o.Key())
}

func (s *BlockService) Close() error {
	log.Debug("blockservice is shutting down...")
	return s.Exchange.Close()
}
