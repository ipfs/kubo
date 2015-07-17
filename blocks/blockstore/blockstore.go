// package blockstore implements a thin wrapper over a datastore, giving a
// clean interface for Getting and Putting block objects.
package blockstore

import (
	"errors"
	"sync"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dsns "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/namespace"
	dsq "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/query"
	mh "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	blocks "github.com/ipfs/go-ipfs/blocks"
	key "github.com/ipfs/go-ipfs/blocks/key"
	eventlog "github.com/ipfs/go-ipfs/thirdparty/eventlog"
)

var log = eventlog.Logger("blockstore")

// BlockPrefix namespaces blockstore datastores
var BlockPrefix = ds.NewKey("blocks")

var ValueTypeMismatch = errors.New("The retrieved value is not a Block")

var ErrNotFound = errors.New("blockstore: block not found")

// Blockstore wraps a Datastore
type Blockstore interface {
	DeleteBlock(key.Key) error
	Has(key.Key) (bool, error)
	Get(key.Key) (*blocks.Block, error)
	Put(*blocks.Block) error
	PutMany([]*blocks.Block) error

	AllKeysChan(ctx context.Context) (<-chan key.Key, error)
}

type GCBlockstore interface {
	Blockstore

	// GCLock locks the blockstore for garbage collection. No operations
	// that expect to finish with a pin should ocurr simultaneously.
	// Reading during GC is safe, and requires no lock.
	GCLock() func()

	// PinLock locks the blockstore for sequences of puts expected to finish
	// with a pin (before GC). Multiple put->pin sequences can write through
	// at the same time, but no GC should not happen simulatenously.
	// Reading during Pinning is safe, and requires no lock.
	PinLock() func()
}

func NewBlockstore(d ds.Batching) *blockstore {
	var dsb ds.Batching
	dd := dsns.Wrap(d, BlockPrefix)
	dsb = dd
	return &blockstore{
		datastore: dsb,
	}
}

type blockstore struct {
	datastore ds.Batching
	// cant be Datastore cause namespace.Datastore doesnt support it.
	// we do check it on `NewBlockstore` though.

	lk sync.RWMutex
}

func (bs *blockstore) Get(k key.Key) (*blocks.Block, error) {
	maybeData, err := bs.datastore.Get(k.DsKey())
	if err == ds.ErrNotFound {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	bdata, ok := maybeData.([]byte)
	if !ok {
		return nil, ValueTypeMismatch
	}

	return blocks.NewBlockWithHash(bdata, mh.Multihash(k))
}

func (bs *blockstore) Put(block *blocks.Block) error {
	k := block.Key().DsKey()

	// Has is cheaper than Put, so see if we already have it
	exists, err := bs.datastore.Has(k)
	if err == nil && exists {
		return nil // already stored.
	}
	return bs.datastore.Put(k, block.Data)
}

func (bs *blockstore) PutMany(blocks []*blocks.Block) error {
	t, err := bs.datastore.Batch()
	if err != nil {
		return err
	}
	for _, b := range blocks {
		k := b.Key().DsKey()
		exists, err := bs.datastore.Has(k)
		if err == nil && exists {
			continue
		}

		err = t.Put(k, b.Data)
		if err != nil {
			return err
		}
	}
	return t.Commit()
}

func (bs *blockstore) Has(k key.Key) (bool, error) {
	return bs.datastore.Has(k.DsKey())
}

func (s *blockstore) DeleteBlock(k key.Key) error {
	return s.datastore.Delete(k.DsKey())
}

// AllKeysChan runs a query for keys from the blockstore.
// this is very simplistic, in the future, take dsq.Query as a param?
//
// AllKeysChan respects context
func (bs *blockstore) AllKeysChan(ctx context.Context) (<-chan key.Key, error) {

	// KeysOnly, because that would be _a lot_ of data.
	q := dsq.Query{KeysOnly: true}
	// datastore/namespace does *NOT* fix up Query.Prefix
	q.Prefix = BlockPrefix.String()
	res, err := bs.datastore.Query(q)
	if err != nil {
		return nil, err
	}

	// this function is here to compartmentalize
	get := func() (k key.Key, ok bool) {
		select {
		case <-ctx.Done():
			return k, false
		case e, more := <-res.Next():
			if !more {
				return k, false
			}
			if e.Error != nil {
				log.Debug("blockstore.AllKeysChan got err:", e.Error)
				return k, false
			}

			// need to convert to key.Key using key.KeyFromDsKey.
			k = key.KeyFromDsKey(ds.NewKey(e.Key))
			log.Debug("blockstore: query got key", k)

			// key must be a multihash. else ignore it.
			_, err := mh.Cast([]byte(k))
			if err != nil {
				return "", true
			}

			return k, true
		}
	}

	output := make(chan key.Key)
	go func() {
		defer func() {
			res.Process().Close() // ensure exit (signals early exit, too)
			close(output)
		}()

		for {
			k, ok := get()
			if !ok {
				return
			}
			if k == "" {
				continue
			}

			select {
			case <-ctx.Done():
				return
			case output <- k:
			}
		}
	}()

	return output, nil
}

func (bs *blockstore) GCLock() func() {
	bs.lk.Lock()
	return bs.lk.Unlock
}

func (bs *blockstore) PinLock() func() {
	bs.lk.RLock()
	return bs.lk.RUnlock
}
