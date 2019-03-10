package coreapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"

	util "github.com/ipfs/go-ipfs/blocks/blockstoreutil"
	pin "github.com/ipfs/go-ipfs/pin"

	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	caopts "github.com/ipfs/interface-go-ipfs-core/options"
)

type BlockAPI CoreAPI

type BlockStat struct {
	path coreiface.ResolvedPath
	size int
}

func (api *BlockAPI) Put(ctx context.Context, src io.Reader, opts ...caopts.BlockPutOption) (coreiface.BlockStat, error) {
	settings, pref, err := caopts.BlockPutOptions(opts...)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(src)
	if err != nil {
		return nil, err
	}

	bcid, err := pref.Sum(data)
	if err != nil {
		return nil, err
	}

	b, err := blocks.NewBlockWithCid(data, bcid)
	if err != nil {
		return nil, err
	}

	if settings.Pin {
		defer api.blockstore.PinLock().Unlock()
	}

	err = api.blocks.AddBlock(b)
	if err != nil {
		return nil, err
	}

	if settings.Pin {
		api.pinning.PinWithMode(b.Cid(), pin.Recursive)
	}

	return &BlockStat{path: coreiface.IpldPath(b.Cid()), size: len(data)}, nil
}

func (api *BlockAPI) Get(ctx context.Context, p coreiface.Path) (io.Reader, error) {
	rp, err := api.core().ResolvePath(ctx, p)
	if err != nil {
		return nil, err
	}

	b, err := api.blocks.GetBlock(ctx, rp.Cid())
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
	cids := []cid.Cid{rp.Cid()}
	o := util.RmBlocksOpts{Force: settings.Force}

	out, err := util.RmBlocks(api.blockstore, api.pinning, cids, o)
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

	b, err := api.blocks.GetBlock(ctx, rp.Cid())
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
