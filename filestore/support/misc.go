package filestore_support

import (
	"errors"
	//ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ipfs/blocks"
	"github.com/ipfs/go-ipfs/commands/files"
	"github.com/ipfs/go-ipfs/merkledag"
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
	// Empty blocks don't have PosInfo, so for now just don't add
	// them to the filestore
	if nd.DataPtr == nil || nd.DataPtr.Size == 0 {
		return b0, nil
	}
	if nd.DataPtr.PosInfo == nil || nd.DataPtr.PosInfo.Stat == nil {
		return nil, errors.New("no file information for block")
	}
	b := &FilestoreBlock{
		BasicBlock: *b0,
		PosInfo:    nd.DataPtr.PosInfo,
		Size:       nd.DataPtr.Size}

	if nd.DataPtr.AltData == nil {
		return b, nil
	}
	d, err := nd.MarshalNoData()
	if err != nil {
		return nil, err
	}
	b.AltData = d
	return b, nil
}

func (NodeToBlock) NeedAltData() bool {
	return true
}
