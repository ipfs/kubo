package filestore_support

import (
	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore"
	dsns "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore/namespace"
	blocks "github.com/ipfs/go-ipfs/blocks"
	bs "github.com/ipfs/go-ipfs/blocks/blockstore"
	fs "github.com/ipfs/go-ipfs/filestore"
)

type blockstore struct {
	bs.GCBlockstore
	datastore [2]ds.Batching
}

func NewBlockstore(b bs.GCBlockstore, d ds.Batching, fs *fs.Datastore) bs.GCBlockstore {
	return &blockstore{b, [2]ds.Batching{dsns.Wrap(d, bs.BlockPrefix), fs}}
}

func (bs *blockstore) Put(block blocks.Block) error {
	k := block.Key().DsKey()

	idx, data := bs.prepareBlock(k, block)
	if data == nil {
		return nil
	}
	return bs.datastore[idx].Put(k, data)
}

func (bs *blockstore) PutMany(blocks []blocks.Block) error {
	var err error
	var t [2]ds.Batch
	for idx, _ := range t {
		t[idx], err = bs.datastore[idx].Batch()
		if err != nil {
			return err
		}
	}
	for _, b := range blocks {
		k := b.Key().DsKey()
		idx, data := bs.prepareBlock(k, b)
		if data == nil {
			continue
		}
		err = t[idx].Put(k, data)
		if err != nil {
			return err
		}
	}
	for idx, _ := range t {
		err := t[idx].Commit()
		if err != nil {
			return err
		}
	}
	return nil
}

func (bs *blockstore) prepareBlock(k ds.Key, block blocks.Block) (int, interface{}) {
	if fsBlock, ok := block.(*FilestoreBlock); !ok {
		//println("Non DataObj")
		// Has is cheaper than Put, so see if we already have it
		exists, err := bs.datastore[0].Has(k)
		if err == nil && exists {
			return 0, nil // already stored.
		}
		return 0, block.Data()
	} else {
		//println("DataObj")
		d := &fs.DataObj{
			FilePath: fsBlock.FullPath,
			Offset:   fsBlock.Offset,
			Size:     fsBlock.Size,
			ModTime:  fs.FromTime(fsBlock.Stat.ModTime()),
		}
		if fsBlock.AltData == nil {
			d.Flags |= fs.Internal
			d.Data = block.Data()
		} else {
			d.Flags |= fs.NoBlockData
			d.Data = fsBlock.AltData
		}
		return 1, d
	}

}
