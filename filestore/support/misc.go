package filestore_support

import (
	"errors"
	//ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ipfs/blocks"
	"github.com/ipfs/go-ipfs/commands/files"
	. "github.com/ipfs/go-ipfs/filestore"
	"github.com/ipfs/go-ipfs/merkledag"
	fs_pb "github.com/ipfs/go-ipfs/unixfs/pb"
)

type FilestoreBlock struct {
	blocks.BasicBlock
	AltData []byte
	*files.PosInfo
	Size uint64
}

type NodeToBlock struct{}

func (NodeToBlock) CreateBlock(nd *merkledag.Node) (blocks.Block, error) {
	//println("filestore create block")
	b0, err := merkledag.CreateBasicBlock(nd)
	if err != nil {
		return nil, err
	}

	altData, fsInfo, err := Reconstruct(b0.Data(), nil, 0)

	if (fsInfo.Type != fs_pb.Data_Raw && fsInfo.Type != fs_pb.Data_File) || fsInfo.FileSize == 0 {
		return b0, nil
	}
	if nd.PosInfo == nil || nd.PosInfo.Stat == nil {
		return nil, errors.New("no file information for block")
	}
	b := &FilestoreBlock{
		BasicBlock: *b0,
		PosInfo:    nd.PosInfo,
		Size:       uint64(fsInfo.FileSize)}

	if len(fsInfo.Data) == 0 {
		return b, nil
	}
	b.AltData = altData
	return b, nil
}
