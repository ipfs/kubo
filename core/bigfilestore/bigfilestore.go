package bigfilestore

import (
	"context"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dsns "github.com/ipfs/go-datastore/namespace"
	dsq "github.com/ipfs/go-datastore/query"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	dshelp "github.com/ipfs/go-ipfs-ds-help"
	logging "github.com/ipfs/go-log"
)

var log = logging.Logger("bigfilestore")

type BigFileStore struct {
	bs         blockstore.Blockstore
	dstore     ds.Batching
	hashOnRead bool
}

// bigFilePrefix namespaces big file datastores
var bigFilePrefix = ds.NewKey("bigfiles")

// NewBigFileStore creates a new bifFileStore
func NewBigFileStore(bstore blockstore.Blockstore, dstore ds.Batching) *BigFileStore {
	return &BigFileStore{
		bs:     bstore,
		dstore: dsns.Wrap(dstore, bigFilePrefix),
	}
}

func (b *BigFileStore) PutBigBlock(streamCid cid.Cid, chunks []*ChunkingManifestChunk) error {
	chunkData, err := serializeChunks(chunks)
	if err != nil {
		return err
	}

	dsk := dshelp.CidToDsKey(streamCid)
	return b.dstore.Put(dsk, chunkData)
}

func (b *BigFileStore) GetBigBlock(streamCid cid.Cid) ([]*ChunkingManifestChunk, error) {
	data, err := b.dstore.Get(dshelp.CidToDsKey(streamCid))
	if err != nil {
		return nil, err
	}

	return deserializeChunks(data)
}

// AllKeysChan returns a channel from which to read the keys stored in
// the blockstore. If the given context is cancelled the channel will be closed.
func (b *BigFileStore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	ctx, cancel := context.WithCancel(ctx)

	a, err := b.bs.AllKeysChan(ctx)
	if err != nil {
		cancel()
		return nil, err
	}

	out := make(chan cid.Cid, dsq.KeysOnlyBufSize)
	go func() {
		defer cancel()
		defer close(out)

		var done bool
		for !done {
			select {
			case c, ok := <-a:
				if !ok {
					done = true
					continue
				}
				select {
				case out <- c:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}

		// Can't do these at the same time because the abstractions around
		// leveldb make us query leveldb for both operations. We apparently
		// cant query leveldb concurrently
		b, err := b.bsAllKeysChan(ctx)
		if err != nil {
			log.Error("error querying filestore: ", err)
			return
		}

		done = false
		for !done {
			select {
			case c, ok := <-b:
				if !ok {
					done = true
					continue
				}
				select {
				case out <- c:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

func (b *BigFileStore) bsAllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	// KeysOnly, because that would be _a lot_ of data.
	q := dsq.Query{KeysOnly: true}
	res, err := b.dstore.Query(q)
	if err != nil {
		return nil, err
	}

	output := make(chan cid.Cid, dsq.KeysOnlyBufSize)
	go func() {
		defer func() {
			res.Close() // ensure exit (signals early exit, too)
			close(output)
		}()

		for {
			e, ok := res.NextSync()
			if !ok {
				return
			}
			if e.Error != nil {
				log.Errorf("blockstore.AllKeysChan got err: %s", e.Error)
				return
			}

			// need to convert to key.Key using key.KeyFromDsKey.
			k, err := dshelp.DsKeyToCid(ds.RawKey(e.Key))
			if err != nil {
				log.Warningf("error parsing key from DsKey: %s", err)
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

// DeleteBlock deletes the block with the given key from the
// blockstore. As expected, in the case of FileManager blocks, only the
// reference is deleted, not its contents. It may return
// ErrNotFound when the block is not stored.
func (b *BigFileStore) DeleteBlock(c cid.Cid) error {
	err1 := b.bs.DeleteBlock(c)
	if err1 != nil && err1 != blockstore.ErrNotFound {
		return err1
	}

	err2 := b.dstore.Delete(dshelp.CidToDsKey(c))
	// if we successfully removed something from the blockstore, but the
	// bigfilestore didnt have it, return success

	switch err2 {
	case nil:
		return nil
	case blockstore.ErrNotFound:
		if err1 == blockstore.ErrNotFound {
			return blockstore.ErrNotFound
		}
		return nil
	default:
		return err2
	}
}

// Get retrieves the block with the given Cid. It may return
// ErrNotFound when the block is not stored.
func (b *BigFileStore) Get(c cid.Cid) (blocks.Block, error) {
	blk, err := b.bs.Get(c)
	switch err {
	case nil:
		return blk, nil
	case blockstore.ErrNotFound:
		chunks, err := b.GetBigBlock(c)
		if err == ds.ErrNotFound {
			return nil, blockstore.ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		var data []byte
		for _, chunk := range chunks {
			blk, err := b.bs.Get(chunk.ChunkCid)
			if err != nil {
				return nil, err
			}
			data = append(data, blk.RawData()...)
		}

		if b.hashOnRead {
			rbcid, err := c.Prefix().Sum(data)
			if err != nil {
				return nil, err
			}

			if !rbcid.Equals(c) {
				return nil, blockstore.ErrHashMismatch
			}
		}

		return blocks.NewBlockWithCid(data, c)
	default:
		return nil, err
	}
}

// GetSize returns the size of the requested block. It may return ErrNotFound
// when the block is not stored.
func (b *BigFileStore) GetSize(c cid.Cid) (int, error) {
	size, err := b.bs.GetSize(c)
	switch err {
	case nil:
		return size, nil
	case blockstore.ErrNotFound:
		chunks, err := b.GetBigBlock(c)
		if err == ds.ErrNotFound {
			return 0, blockstore.ErrNotFound
		}
		if err != nil {
			return 0, err
		}
		sz := 0
		for _, chunk := range chunks {
			sz += int(chunk.Size)
		}
		return sz, nil
	default:
		return -1, err
	}
}

// Has returns true if the block with the given Cid is
// stored in the Filestore.
func (b *BigFileStore) Has(c cid.Cid) (bool, error) {
	has, err := b.bs.Has(c)
	if err != nil {
		return false, err
	}

	if has {
		return true, nil
	}

	return b.dstore.Has(dshelp.CidToDsKey(c))
}

// Put stores a block in the Filestore. For blocks of
// underlying type FilestoreNode, the operation is
// delegated to the FileManager, while the rest of blocks
// are handled by the regular blockstore.
func (b *BigFileStore) Put(blk blocks.Block) error {
	has, err := b.Has(blk.Cid())
	if err != nil {
		return err
	}

	if has {
		return nil
	}

	return b.bs.Put(blk)
}

// PutMany is like Put(), but takes a slice of blocks, allowing
// the underlying blockstore to perform batch transactions.
func (b *BigFileStore) PutMany(bs []blocks.Block) error {
	return b.bs.PutMany(bs)
}

// HashOnRead calls blockstore.HashOnRead.
func (b *BigFileStore) HashOnRead(enabled bool) {
	b.bs.HashOnRead(enabled)
	b.hashOnRead = true
}

var _ blockstore.Blockstore = (*BigFileStore)(nil)
