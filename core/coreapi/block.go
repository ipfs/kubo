package coreapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	util "github.com/ipfs/go-ipfs/blocks/blockstoreutil"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	blocks "gx/ipfs/Qmej7nf81hi2x2tvjRBF3mcp74sQyuDH4VMYDGd1YtXjb2/go-block-format"
)

type BlockAPI CoreAPI

type BlockStat struct {
	path coreiface.Path
	size int
}

func (api *BlockAPI) Put(ctx context.Context, src io.Reader, opts ...caopts.BlockPutOption) (coreiface.Path, error) {
	settings, err := caopts.BlockPutOptions(opts...)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(src)
	if err != nil {
		return nil, err
	}

	var pref cid.Prefix
	pref.Version = 1

	formatval, ok := cid.Codecs[settings.Codec]
	if !ok {
		return nil, fmt.Errorf("unrecognized format: %s", settings.Codec)
	}
	if settings.Codec == "v0" && settings.MhType == mh.SHA2_256 {
		pref.Version = 0
	}
	pref.Codec = formatval

	pref.MhType = settings.MhType
	pref.MhLength = settings.MhLength

	bcid, err := pref.Sum(data)
	if err != nil {
		return nil, err
	}

	b, err := blocks.NewBlockWithCid(data, bcid)
	if err != nil {
		return nil, err
	}

	err = api.node.Blocks.AddBlock(b)
	if err != nil {
		return nil, err
	}

	return ParseCid(b.Cid()), nil
}

func (api *BlockAPI) Get(ctx context.Context, p coreiface.Path) (io.Reader, error) {
	b, err := api.node.Blocks.GetBlock(ctx, p.Cid())
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(b.RawData()), nil
}

func (api *BlockAPI) Rm(ctx context.Context, p coreiface.Path, opts ...caopts.BlockRmOption) error {
	settings, err := caopts.BlockRmOptions(opts...)
	if err != nil {
		return err
	}
	cids := []*cid.Cid{p.Cid()}
	o := util.RmBlocksOpts{Force: settings.Force}

	out, err := util.RmBlocks(api.node.Blockstore, api.node.Pinning, cids, o)
	if err != nil {
		return err
	}

	select {
	case res, ok := <-out:
		if !ok {
			return nil
		}

		remBlock, ok := res.(*util.RemovedBlock)
		if !ok {
			return errors.New("got unexpected output from util.RmBlocks")
		}

		if remBlock.Error != "" {
			return errors.New(remBlock.Error)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (api *BlockAPI) Stat(ctx context.Context, p coreiface.Path) (coreiface.BlockStat, error) {
	b, err := api.node.Blocks.GetBlock(ctx, p.Cid())
	if err != nil {
		return nil, err
	}

	return &BlockStat{
		path: ParseCid(b.Cid()),
		size: len(b.RawData()),
	}, nil
}

func (bs *BlockStat) Size() int {
	return bs.size
}

func (bs *BlockStat) Path() coreiface.Path {
	return bs.path
}
