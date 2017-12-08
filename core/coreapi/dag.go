package coreapi

import (
	"context"
	"fmt"
	"io"

	gopath "path"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	coredag "github.com/ipfs/go-ipfs/core/coredag"

	cid "gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
	mh "gx/ipfs/QmU9a9NV9RdPNwZQDYd5uKsm6N6LJLSvLbywDDYFbaaC6P/go-multihash"
)

type DagAPI CoreAPI

func (api *DagAPI) Put(ctx context.Context, src io.Reader, inputEnc string, format *cid.Prefix) ([]coreiface.Node, error) {
	if format == nil {
		format = &cid.Prefix{
			Version:  1,
			Codec:    cid.DagCBOR,
			MhType:   mh.SHA2_256,
			MhLength: mh.DefaultLengths[mh.SHA2_256],
		}
	}

	codec, ok := cid.CodecToStr[format.Codec]
	if !ok {
		return nil, fmt.Errorf("invalid codec %d", format.Codec)
	}

	nds, err := coredag.ParseInputs(inputEnc, codec, src, format.MhType, format.MhLength)
	if err != nil {
		return nil, err
	}
	if len(nds) == 0 {
		return nil, fmt.Errorf("no node returned from ParseInputs")
	}

	out := make([]coreiface.Node, len(nds))
	for n, nd := range nds {
		_, err := api.node.DAG.Add(nd)
		if err != nil {
			return nil, err
		}
		out[n] = nd
	}

	return out, nil
}

func (api *DagAPI) Get(ctx context.Context, path coreiface.Path) (coreiface.Node, error) {
	return api.core().ResolveNode(ctx, path)
}

func (api *DagAPI) Tree(ctx context.Context, p coreiface.Path, depth int) ([]coreiface.Path, error) {
	n, err := api.Get(ctx, p)
	if err != nil {
		return nil, err
	}
	paths := n.Tree("", depth)
	out := make([]coreiface.Path, len(paths))
	for n, p2 := range paths {
		out[n], err = ParsePath(gopath.Join(p.String(), p2))
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}

func (api *DagAPI) core() coreiface.CoreAPI {
	return (*CoreAPI)(api)
}
