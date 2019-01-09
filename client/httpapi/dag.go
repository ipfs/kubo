package httpapi

import (
	"context"
	"fmt"
	"io"
	"math"

	"github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipld-format"
	mh "github.com/multiformats/go-multihash"
)

type DagAPI HttpApi

func (api *DagAPI) Put(ctx context.Context, src io.Reader, opts ...caopts.DagPutOption) (iface.ResolvedPath, error) {
	options, err := caopts.DagPutOptions(opts...)
	if err != nil {
		return nil, err
	}

	codec, ok := cid.CodecToStr[options.Codec]
	if !ok {
		return nil, fmt.Errorf("unknowm codec %d", options.MhType)
	}

	if options.MhLength != -1 {
		return nil, fmt.Errorf("setting hash len is not supported yet")
	}

	var out struct {
		Cid cid.Cid
	}
	req := api.core().request("dag/put").
		Option("format", codec).
		Option("input-enc", options.InputEnc)

	if options.MhType != math.MaxUint64 {
		mht, ok := mh.Codes[options.MhType]
		if !ok {
			return nil, fmt.Errorf("unknowm mhType %d", options.MhType)
		}
		req.Option("hash", mht)
	}

	err = req.FileBody(src).Exec(ctx, &out)
	if err != nil {
		return nil, err
	}

	return iface.IpldPath(out.Cid), nil
}

func (api *DagAPI) Get(ctx context.Context, path iface.Path) (format.Node, error) {
	panic("implement me")
}

func (api *DagAPI) Tree(ctx context.Context, path iface.Path, opts ...caopts.DagTreeOption) ([]iface.Path, error) {
	panic("implement me")
}

func (api *DagAPI) Batch(ctx context.Context) iface.DagBatch {
	panic("implement me")
}

func (api *DagAPI) core() *HttpApi {
	return (*HttpApi)(api)
}
