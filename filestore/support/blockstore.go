package filestore_support

import (
	"errors"
	b "github.com/ipfs/go-ipfs/blocks"
	bs "github.com/ipfs/go-ipfs/blocks/blockstore"
	. "github.com/ipfs/go-ipfs/filestore"
	fs_pb "github.com/ipfs/go-ipfs/unixfs/pb"
	ds "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore"
)

type blockstore struct {
	bs.GCBlockstore
	filestore *Datastore
}

func NewBlockstore(b bs.GCBlockstore, fs *Datastore) bs.GCBlockstore {
	return &blockstore{b, fs}
}

func (bs *blockstore) Put(block b.Block) error {
	k := block.Key().DsKey()

	data, err := bs.prepareBlock(k, block)
	if err != nil {
		return err
	} else if data == nil {
		return bs.GCBlockstore.Put(block)
	}
	return bs.filestore.Put(k, data)
}

func (bs *blockstore) PutMany(blocks []b.Block) error {
	var nonFilestore []b.Block

	t, err := bs.filestore.Batch()
	if err != nil {
		return err
	}

	for _, b := range blocks {
		k := b.Key().DsKey()
		data, err := bs.prepareBlock(k, b)
		if err != nil {
			return err
		} else if data == nil {
			nonFilestore = append(nonFilestore, b)
			continue
		} 

		err = t.Put(k, data)
		if err != nil {
			return err
		}
	}

	err = t.Commit()
	if err != nil {
		return err
	}

	if len(nonFilestore) > 0 {
		return bs.GCBlockstore.PutMany(nonFilestore)
	} else {
		return nil
	}
}

func (bs *blockstore) prepareBlock(k ds.Key, block b.Block) (*DataObj, error) {
	altData, fsInfo, err := Reconstruct(block.Data(), nil, 0)
	if err != nil {
		return nil, err
	}

	if (fsInfo.Type != fs_pb.Data_Raw && fsInfo.Type != fs_pb.Data_File) || fsInfo.FileSize == 0 {
		// If the node does not contain file data store using
		// the normal datastore and not the filestore.
		// Also don't use the filestore if the filesize is 0
		// (i.e. an empty file) as posInfo might be nil.
		return nil, nil
	} else {
		posInfo := block.PosInfo()
		if posInfo == nil || posInfo.Stat == nil {
			return nil, errors.New("no file information for block")
		}
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
		return d, nil
	}

}
