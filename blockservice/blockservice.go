// package blockservice implements a BlockService interface that provides
// a single GetBlock/AddBlock interface that seamlessly retrieves data either
// locally or from a remote peer through the exchange.
package blockservice

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/ipfs/go-ipfs/thirdparty/verifcid"

	logging "gx/ipfs/QmTG23dvpBCBjqQwyDxV8CQT6jmS4PSftNr1VqHhE3MLy7/go-log"
	blockstore "gx/ipfs/QmayRSLCiM2gWR7Kay8vqu3Yy5mf7yPqocF9ZRgDUPYMcc/go-ipfs-blockstore"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	exchange "gx/ipfs/QmdcAXgEHUueP4A7b5hjabKn2EooeHgMreMvFC249dGCgc/go-ipfs-exchange-interface"
	blocks "gx/ipfs/Qmej7nf81hi2x2tvjRBF3mcp74sQyuDH4VMYDGd1YtXjb2/go-block-format"
)

var log = logging.Logger("blockservice")

var ErrNotFound = errors.New("blockservice: key not found")

// BlockGetter is the common interface shared between blockservice sessions and
// the blockservice.
type BlockGetter interface {
	// GetBlock gets the requested block.
	GetBlock(ctx context.Context, c *cid.Cid) (blocks.Block, error)

	// GetBlocks does a batch request for the given cids, returning blocks as
	// they are found, in no particular order.
	//
	// It may not be able to find all requested blocks (or the context may
	// be canceled). In that case, it will close the channel early. It is up
	// to the consumer to detect this situation and keep track which blocks
	// it has received and which it hasn't.
	GetBlocks(ctx context.Context, ks []*cid.Cid) <-chan blocks.Block
}

// BlockService is a hybrid block datastore. It stores data in a local
// datastore and may retrieve data from a remote Exchange.
// It uses an internal `datastore.Datastore` instance to store values.
type BlockService interface {
	io.Closer
	BlockGetter

	// Blockstore returns a reference to the underlying blockstore
	Blockstore() blockstore.Blockstore

	// Exchange returns a reference to the underlying exchange (usually bitswap)
	Exchange() exchange.Interface

	// AddBlock puts a given block to the underlying datastore
	AddBlock(o blocks.Block) error

	// AddBlocks adds a slice of blocks at the same time using batching
	// capabilities of the underlying datastore whenever possible.
	AddBlocks(bs []blocks.Block) error

	// DeleteBlock deletes the given block from the blockservice.
	DeleteBlock(o *cid.Cid) error
}

type blockService struct {
	blockstore blockstore.Blockstore
	exchange   exchange.Interface
	// If checkFirst is true then first check that a block doesn't
	// already exist to avoid republishing the block on the exchange.
	checkFirst bool
}

// NewBlockService creates a BlockService with given datastore instance.
func New(bs blockstore.Blockstore, rem exchange.Interface) BlockService {
	if rem == nil {
		log.Warning("blockservice running in local (offline) mode.")
	}

	return &blockService{
		blockstore: bs,
		exchange:   rem,
		checkFirst: true,
	}
}

// NewWriteThrough ceates a BlockService that guarantees writes will go
// through to the blockstore and are not skipped by cache checks.
func NewWriteThrough(bs blockstore.Blockstore, rem exchange.Interface) BlockService {
	if rem == nil {
		log.Warning("blockservice running in local (offline) mode.")
	}

	return &blockService{
		blockstore: bs,
		exchange:   rem,
		checkFirst: false,
	}
}

// Blockstore returns the blockstore behind this blockservice.
func (s *blockService) Blockstore() blockstore.Blockstore {
	return s.blockstore
}

// Exchange returns the exchange behind this blockservice.
func (s *blockService) Exchange() exchange.Interface {
	return s.exchange
}

// NewSession creates a new session that allows for
// controlled exchange of wantlists to decrease the bandwidth overhead.
// If the current exchange is a SessionExchange, a new exchange
// session will be created. Otherwise, the current exchange will be used
// directly.
func NewSession(ctx context.Context, bs BlockService) *Session {
	exch := bs.Exchange()
	if sessEx, ok := exch.(exchange.SessionExchange); ok {
		ses := sessEx.NewSession(ctx)
		return &Session{
			ses: ses,
			bs:  bs.Blockstore(),
		}
	}
	return &Session{
		ses: exch,
		bs:  bs.Blockstore(),
	}
}

// AddBlock adds a particular block to the service, Putting it into the datastore.
// TODO pass a context into this if the remote.HasBlock is going to remain here.
func (s *blockService) AddBlock(o blocks.Block) error {
	c := o.Cid()
	// hash security
	err := verifcid.ValidateCid(c)
	if err != nil {
		return err
	}
	if s.checkFirst {
		if has, err := s.blockstore.Has(c); has || err != nil {
			return err
		}
	}

	if err := s.blockstore.Put(o); err != nil {
		return err
	}

	log.Event(context.TODO(), "BlockService.BlockAdded", c)

	if err := s.exchange.HasBlock(o); err != nil {
		// TODO(#4623): really an error?
		return errors.New("blockservice is closed")
	}

	return nil
}

func (s *blockService) AddBlocks(bs []blocks.Block) error {
	// hash security
	for _, b := range bs {
		err := verifcid.ValidateCid(b.Cid())
		if err != nil {
			return err
		}
	}
	var toput []blocks.Block
	if s.checkFirst {
		toput = make([]blocks.Block, 0, len(bs))
		for _, b := range bs {
			has, err := s.blockstore.Has(b.Cid())
			if err != nil {
				return err
			}
			if !has {
				toput = append(toput, b)
			}
		}
	} else {
		toput = bs
	}

	err := s.blockstore.PutMany(toput)
	if err != nil {
		return err
	}

	for _, o := range toput {
		log.Event(context.TODO(), "BlockService.BlockAdded", o.Cid())
		if err := s.exchange.HasBlock(o); err != nil {
			// TODO(#4623): Should this really *return*?
			return fmt.Errorf("blockservice is closed (%s)", err)
		}
	}
	return nil
}

// GetBlock retrieves a particular block from the service,
// Getting it from the datastore using the key (hash).
func (s *blockService) GetBlock(ctx context.Context, c *cid.Cid) (blocks.Block, error) {
	log.Debugf("BlockService GetBlock: '%s'", c)

	var f exchange.Fetcher
	if s.exchange != nil {
		f = s.exchange
	}

	return getBlock(ctx, c, s.blockstore, f) // hash security
}

func getBlock(ctx context.Context, c *cid.Cid, bs blockstore.Blockstore, f exchange.Fetcher) (blocks.Block, error) {
	err := verifcid.ValidateCid(c) // hash security
	if err != nil {
		return nil, err
	}

	block, err := bs.Get(c)
	if err == nil {
		return block, nil
	}

	if err == blockstore.ErrNotFound && f != nil {
		// TODO be careful checking ErrNotFound. If the underlying
		// implementation changes, this will break.
		log.Debug("Blockservice: Searching bitswap")
		blk, err := f.GetBlock(ctx, c)
		if err != nil {
			if err == blockstore.ErrNotFound {
				return nil, ErrNotFound
			}
			return nil, err
		}
		log.Event(ctx, "BlockService.BlockFetched", c)
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
	return getBlocks(ctx, ks, s.blockstore, s.exchange) // hash security
}

func getBlocks(ctx context.Context, ks []*cid.Cid, bs blockstore.Blockstore, f exchange.Fetcher) <-chan blocks.Block {
	out := make(chan blocks.Block)

	go func() {
		defer close(out)

		k := 0
		for _, c := range ks {
			// hash security
			if err := verifcid.ValidateCid(c); err == nil {
				ks[k] = c
				k++
			} else {
				log.Errorf("unsafe CID (%s) passed to blockService.GetBlocks: %s", c, err)
			}
		}
		ks = ks[:k]

		var misses []*cid.Cid
		for _, c := range ks {
			hit, err := bs.Get(c)
			if err != nil {
				misses = append(misses, c)
				continue
			}
			select {
			case out <- hit:
			case <-ctx.Done():
				return
			}
		}

		if len(misses) == 0 {
			return
		}

		rblocks, err := f.GetBlocks(ctx, misses)
		if err != nil {
			log.Debugf("Error with GetBlocks: %s", err)
			return
		}

		for b := range rblocks {
			log.Event(ctx, "BlockService.BlockFetched", b.Cid())
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
func (s *blockService) DeleteBlock(c *cid.Cid) error {
	err := s.blockstore.DeleteBlock(c)
	if err == nil {
		log.Event(context.TODO(), "BlockService.BlockDeleted", c)
	}
	return err
}

func (s *blockService) Close() error {
	log.Debug("blockservice is shutting down...")
	return s.exchange.Close()
}

// Session is a helper type to provide higher level access to bitswap sessions
type Session struct {
	bs  blockstore.Blockstore
	ses exchange.Fetcher
}

// GetBlock gets a block in the context of a request session
func (s *Session) GetBlock(ctx context.Context, c *cid.Cid) (blocks.Block, error) {
	return getBlock(ctx, c, s.bs, s.ses) // hash security
}

// GetBlocks gets blocks in the context of a request session
func (s *Session) GetBlocks(ctx context.Context, ks []*cid.Cid) <-chan blocks.Block {
	return getBlocks(ctx, ks, s.bs, s.ses) // hash security
}

var _ BlockGetter = (*Session)(nil)
