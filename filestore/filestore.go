package filestore

import (
	"context"

	"github.com/ipfs/go-ipfs/blocks"
	"github.com/ipfs/go-ipfs/blocks/blockstore"
	posinfo "github.com/ipfs/go-ipfs/thirdparty/posinfo"

	dsq "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore/query"
	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	cid "gx/ipfs/QmV5gPoRsjN1Gid3LMdNZTyfCtP2DsvqEbMAmz82RmmiGk/go-cid"
)

var log = logging.Logger("filestore")

type Filestore struct {
	fm *FileManager
	bs blockstore.Blockstore
}

func NewFilestore(bs blockstore.Blockstore, fm *FileManager) *Filestore {
	return &Filestore{fm, bs}
}

func (f *Filestore) AllKeysChan(ctx context.Context) (<-chan *cid.Cid, error) {
	ctx, cancel := context.WithCancel(ctx)

	a, err := f.bs.AllKeysChan(ctx)
	if err != nil {
		return nil, err
	}

	out := make(chan *cid.Cid, dsq.KeysOnlyBufSize)
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
		b, err := f.fm.AllKeysChan(ctx)
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

func (f *Filestore) DeleteBlock(c *cid.Cid) error {
	err1 := f.bs.DeleteBlock(c)
	if err1 != nil && err1 != blockstore.ErrNotFound {
		return err1
	}

	err2 := f.fm.DeleteBlock(c)
	// if we successfully removed something from the blockstore, but the
	// filestore didnt have it, return success

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

func (f *Filestore) Get(c *cid.Cid) (blocks.Block, error) {
	blk, err := f.bs.Get(c)
	switch err {
	default:
		return nil, err
	case nil:
		return blk, nil
	case blockstore.ErrNotFound:
		// try filestore
	}

	return f.fm.Get(c)
}

func (f *Filestore) Has(c *cid.Cid) (bool, error) {
	has, err := f.bs.Has(c)
	if err != nil {
		return false, err
	}

	if has {
		return true, nil
	}

	return f.fm.Has(c)
}

func (f *Filestore) Put(b blocks.Block) error {
	has, err := f.Has(b.Cid())
	if err != nil {
		return err
	}

	if has {
		return nil
	}

	switch b := b.(type) {
	case *posinfo.FilestoreNode:
		return f.fm.Put(b)
	default:
		return f.bs.Put(b)
	}
}

func (f *Filestore) PutMany(bs []blocks.Block) error {
	var normals []blocks.Block
	var fstores []*posinfo.FilestoreNode

	for _, b := range bs {
		has, err := f.Has(b.Cid())
		if err != nil {
			return err
		}

		if has {
			continue
		}

		switch b := b.(type) {
		case *posinfo.FilestoreNode:
			fstores = append(fstores, b)
		default:
			normals = append(normals, b)
		}
	}

	if len(normals) > 0 {
		err := f.bs.PutMany(normals)
		if err != nil {
			return err
		}
	}

	if len(fstores) > 0 {
		err := f.fm.PutMany(fstores)
		if err != nil {
			return err
		}
	}
	return nil
}

var _ blockstore.Blockstore = (*Filestore)(nil)
