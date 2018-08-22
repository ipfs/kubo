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

	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	blocks "gx/ipfs/QmWAzSEoqZ6xU6pu8yL8e5WaMb7wtbfbhhN4p1DknUPtr3/go-block-format"
	cid "gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
)

type BlockAPI CoreAPI

type BlockStat struct {
	path coreiface.ResolvedPath
	size int
}

func (api *BlockAPI) Put(ctx context.Context, src io.Reader, opts ...caopts.BlockPutOption) (coreiface.ResolvedPath, error) {
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

	return coreiface.IpldPath(b.Cid()), nil
}

func (api *BlockAPI) Get(ctx context.Context, p coreiface.Path) (io.Reader, error) {
	rp, err := api.core().ResolvePath(ctx, p)
	if err != nil {
		return nil, err
	}

	b, err := api.node.Blocks.GetBlock(ctx, rp.Cid())
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(b.RawData()), nil
}

func (api *BlockAPI) Rm(ctx context.Context, p coreiface.Path, opts ...caopts.BlockRmOption) error {
	rp, err := api.core().ResolvePath(ctx, p)
	if err != nil {
		return err
	}

	settings, err := caopts.BlockRmOptions(opts...)
	if err != nil {
		return err
	}
	cids := []*cid.Cid{rp.Cid()}
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
	rp, err := api.core().ResolvePath(ctx, p)
	if err != nil {
		return nil, err
	}

	b, err := api.node.Blocks.GetBlock(ctx, rp.Cid())
	if err != nil {
		return nil, err
	}

	return &BlockStat{
		path: coreiface.IpldPath(b.Cid()),
		size: len(b.RawData()),
	}, nil
}

func (bs *BlockStat) Size() int {
	return bs.size
}

func (bs *BlockStat) Path() coreiface.ResolvedPath {
	return bs.path
}

func (api *BlockAPI) core() coreiface.CoreAPI {
	return (*CoreAPI)(api)
}
