// package blockstore implements a thin wrapper over a datastore, giving a
// clean interface for Getting and Putting block objects.
package blockstore

import (
	"errors"
	"sync"
	"sync/atomic"

	blocks "github.com/ipfs/go-ipfs/blocks"
	key "github.com/ipfs/go-ipfs/blocks/key"
	logging "gx/ipfs/QmNQynaz7qfriSUJkiEZUrm2Wen1u3Kj9goZzWtrPyu7XR/go-log"
	ds "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore"
	dsns "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore/namespace"
	dsq "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore/query"
	mh "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

var log = logging.Logger("blockstore")

// BlockPrefix namespaces blockstore datastores
const DefaultPrefix = "/blocks"
var blockPrefix = ds.NewKey(DefaultPrefix)

var ValueTypeMismatch = errors.New("the retrieved value is not a Block")
var ErrHashMismatch = errors.New("block in storage has different hash than requested")

var ErrNotFound = errors.New("blockstore: block not found")

// Blockstore wraps a Datastore
type Blockstore interface {
	DeleteBlock(key.Key) error
	Has(key.Key) (bool, error)
	Get(key.Key) (blocks.Block, error)
	Put(blocks.Block) error
	PutMany([]blocks.Block) error

	AllKeysChan(ctx context.Context) (<-chan key.Key, error)
}

type GCLocker interface {
	// GCLock locks the blockstore for garbage collection. No operations
	// that expect to finish with a pin should ocurr simultaneously.
	// Reading during GC is safe, and requires no lock.
	GCLock() Unlocker

	// PinLock locks the blockstore for sequences of puts expected to finish
	// with a pin (before GC). Multiple put->pin sequences can write through
	// at the same time, but no GC should not happen simulatenously.
	// Reading during Pinning is safe, and requires no lock.
	PinLock() Unlocker

	// GcRequested returns true if GCLock has been called and is waiting to
	// take the lock
	GCRequested() bool
}

type GCBlockstore interface {
	Blockstore
	GCLocker
}

func NewGCBlockstore (bs Blockstore, gcl GCLocker) GCBlockstore {
	return gcBlockstore {bs,gcl}
}

type gcBlockstore struct {
	Blockstore
	GCLocker
}

func NewBlockstore(d ds.Batching) *blockstore {
	return NewBlockstoreWPrefix(d, "")
}

func NewBlockstoreWPrefix(d ds.Batching, prefix string) *blockstore {
	if prefix == "" {
		prefix = DefaultPrefix
	}
	var dsb ds.Batching
	prefixKey := ds.NewKey(prefix)
	dd := dsns.Wrap(d, prefixKey)
	dsb = dd
	return &blockstore{
		datastore: dsb,
		prefix:    prefixKey,
	}
}

type blockstore struct {
	datastore ds.Batching
	prefix    ds.Key

	rehash bool
}

func (bs *blockstore) RuntimeHashing(enabled bool) {
	bs.rehash = enabled
}

func (bs *blockstore) Get(k key.Key) (blocks.Block, error) {
	if k == "" {
		return nil, ErrNotFound
	}

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

	if bs.rehash {
		rb := blocks.NewBlock(bdata)
		if rb.Key() != k {
			return nil, ErrHashMismatch
		} else {
			return rb, nil
		}
	} else {
		return blocks.NewBlockWithHash(bdata, mh.Multihash(k))
	}
}

func (bs *blockstore) Put(block blocks.Block) error {
	k := block.Key().DsKey()

	// Note: The Has Check is now done by the MultiBlockstore

	return bs.datastore.Put(k, block.Data())
}

func (bs *blockstore) PutMany(blocks []blocks.Block) error {
	t, err := bs.datastore.Batch()
	if err != nil {
		return err
	}
	for _, b := range blocks {
		k := b.Key().DsKey()
		err = t.Put(k, b.Data())
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
	q.Prefix = bs.prefix.String()
	res, err := bs.datastore.Query(q)
	if err != nil {
		return nil, err
	}

	// this function is here to compartmentalize
	get := func() (key.Key, bool) {
		select {
		case <-ctx.Done():
			return "", false
		case e, more := <-res.Next():
			if !more {
				return "", false
			}
			if e.Error != nil {
				log.Debug("blockstore.AllKeysChan got err:", e.Error)
				return "", false
			}

			// need to convert to key.Key using key.KeyFromDsKey.
			k, err := key.KeyFromDsKey(ds.NewKey(e.Key))
			if err != nil {
				log.Warningf("error parsing key from DsKey: ", err)
				return "", true
			}
			log.Debug("blockstore: query got key", k)

			// key must be a multihash. else ignore it.
			_, err = mh.Cast([]byte(k))
			if err != nil {
				log.Warningf("key from datastore was not a multihash: ", err)
				return "", true
			}

			return k, true
		}
	}

	output := make(chan key.Key, dsq.KeysOnlyBufSize)
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

type gclocker struct {
	lk      sync.RWMutex
	gcreq   int32
	gcreqlk sync.Mutex
}

type Unlocker interface {
	Unlock()
}

type unlocker struct {
	unlock func()
}

func (u *unlocker) Unlock() {
	u.unlock()
	u.unlock = nil // ensure its not called twice
}

func (bs *gclocker) GCLock() Unlocker {
	atomic.AddInt32(&bs.gcreq, 1)
	bs.lk.Lock()
	atomic.AddInt32(&bs.gcreq, -1)
	return &unlocker{bs.lk.Unlock}
}

func (bs *gclocker) PinLock() Unlocker {
	bs.lk.RLock()
	return &unlocker{bs.lk.RUnlock}
}

func (bs *gclocker) GCRequested() bool {
	return atomic.LoadInt32(&bs.gcreq) > 0
}
