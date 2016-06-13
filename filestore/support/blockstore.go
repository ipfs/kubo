package filestore_support

import (
	"errors"
	blocks "github.com/ipfs/go-ipfs/blocks"
	bs "github.com/ipfs/go-ipfs/blocks/blockstore"
	. "github.com/ipfs/go-ipfs/filestore"
	fs_pb "github.com/ipfs/go-ipfs/unixfs/pb"
	ds "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore"
	dsns "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore/namespace"
)

type blockstore struct {
	bs.GCBlockstore
	datastore [2]ds.Batching
}

func NewBlockstore(b bs.GCBlockstore, d ds.Batching, fs *Datastore) bs.GCBlockstore {
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
	altData, fsInfo, err := Reconstruct(block.Data(), nil, 0)
	if err != nil {
		return 0, err
	}

	if (fsInfo.Type != fs_pb.Data_Raw && fsInfo.Type != fs_pb.Data_File) || fsInfo.FileSize == 0 {
		//println("Non DataObj")
		// Has is cheaper than Put, so see if we already have it
		exists, err := bs.datastore[0].Has(k)
		if err == nil && exists {
			return 0, nil // already stored.
		}
		return 0, block.Data()
	} else {
		posInfo := block.PosInfo()
		if posInfo == nil || posInfo.Stat == nil {
			return 0, errors.New("no file information for block")
		}
		//println("DataObj")
		d := &DataObj{
			FilePath: CleanPath(posInfo.FullPath),
			Offset:   posInfo.Offset,
			Size:     uint64(fsInfo.FileSize),
			ModTime:  FromTime(posInfo.Stat.ModTime()),
		}
		if len(fsInfo.Data) == 0 {
			d.Flags |= Internal
			d.Data = block.Data()
		} else {
			d.Flags |= NoBlockData
			d.Data = altData
		}
		return 1, d
	}

}
