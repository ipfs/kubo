package filestore_support

import (
	//ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore"
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
	if nd.DataPtr == nil {
		return b0, nil
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
