package httpapi

import (
	"context"
	"io"

	"github.com/ipfs/go-ipfs/core/coreapi/interface"
	"github.com/ipfs/go-ipfs/core/coreapi/interface/options"
)

type BlockAPI HttpApi

func (api *BlockAPI) Put(ctx context.Context, r io.Reader, opts ...options.BlockPutOption) (iface.BlockStat, error) {
	return nil, ErrNotImplemented
}

func (api *BlockAPI) Get(ctx context.Context, p iface.Path) (io.Reader, error) {
	resp, err := api.core().request("block/get", p.String()).Send(context.Background())
	if err != nil {
		return nil, err
	}

	//TODO: is close on the reader enough?
	//defer resp.Close()

	//TODO: make blockApi return ReadCloser
	return resp.Output, resp.Error
}

func (api *BlockAPI) Rm(ctx context.Context, p iface.Path, opts ...options.BlockRmOption) error {
	return ErrNotImplemented
}

func (api *BlockAPI) Stat(ctx context.Context, p iface.Path) (iface.BlockStat, error) {
	return nil, ErrNotImplemented
}

func (api *BlockAPI) core() *HttpApi {
	return (*HttpApi)(api)
}
