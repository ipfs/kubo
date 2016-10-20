package filestore_support

import (
	"fmt"
	b "github.com/ipfs/go-ipfs/blocks"
	BS "github.com/ipfs/go-ipfs/blocks/blockstore"
	. "github.com/ipfs/go-ipfs/filestore"
	pi "github.com/ipfs/go-ipfs/thirdparty/posinfo"
	fs_pb "github.com/ipfs/go-ipfs/unixfs/pb"
	cid "gx/ipfs/QmXfiyr2RWEXpVDdaYnD2HNiBk6UBddsvEP4RPfXb6nGqY/go-cid"
	ds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"

	"gx/ipfs/QmRpAnJ1Mvd2wCtwoFevW8pbLTivUqmFxynptG6uvp1jzC/safepath"
)

type blockstore struct {
	BS.GCBlockstore
	filestore *Datastore
}

func NewBlockstore(b BS.GCBlockstore, fs *Datastore) BS.GCBlockstore {
	return &blockstore{b, fs}
}

func (bs *blockstore) Put(block b.Block) error {
	k := BS.CidToDsKey(block.Cid())

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
		k := BS.CidToDsKey(b.Cid())
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
		err := bs.GCBlockstore.PutMany(nonFilestore)
		if err != nil {
			return err
		}
		return nil
	} else {
		return nil
	}
}

func (bs *blockstore) prepareBlock(k ds.Key, block b.Block) (*DataObj, error) {
	altData, fsInfo, err := decode(block)
	if err != nil {
		return nil, err
	}

	var fileSize uint64
	if fsInfo == nil {
		fileSize = uint64(len(block.RawData()))
	} else {
		fileSize = fsInfo.FileSize
	}

	if fsInfo != nil && fsInfo.Type != fs_pb.Data_Raw && fsInfo.Type != fs_pb.Data_File {
		// If the node does not contain file data store using
		// the normal datastore and not the filestore.
		return nil, nil
	} else if fileSize == 0 {
		// Special case for empty files as the block doesn't
		// have any file information associated with it
		return &DataObj{
			FilePath: "",
			Offset:   0,
			Size:     0,
			ModTime:  0,
			Flags:    Internal | WholeFile,
			Data:     block.RawData(),
		}, nil
	} else {
		fsn, ok := block.(*pi.FilestoreNode)
		if !ok {
			return nil, fmt.Errorf("%s: no file information for block", block.Cid())
		}
		posInfo := fsn.PosInfo
		if posInfo.Stat == nil {
			return nil, fmt.Errorf("%s: %s: no stat information for file", block.Cid(), posInfo.FullPath)
		}
		d := &DataObj{
			FilePath: safepath.Clean(posInfo.FullPath),
			Offset:   posInfo.Offset,
			Size:     uint64(fileSize),
			ModTime:  FromTime(posInfo.Stat.ModTime()),
		}
		if fsInfo == nil {
			d.Flags |= NoBlockData
			d.Data = nil
		} else if len(fsInfo.Data) == 0 {
			d.Flags |= Internal
			d.Data = block.RawData()
		} else {
			d.Flags |= NoBlockData
			d.Data = altData
		}
		return d, nil
	}

}

func decode(block b.Block) ([]byte, *UnixFSInfo, error) {
	switch block.Cid().Type() {
	case cid.Protobuf:
		return Reconstruct(block.RawData(), nil, 0)
	case cid.Raw:
		return nil, nil, nil
	default:
		return nil, nil, fmt.Errorf("unsupported block type")
	}
}
