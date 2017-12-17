package coreapi

import (
	"context"
	"fmt"
	"io"

	gopath "path"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
	coredag "github.com/ipfs/go-ipfs/core/coredag"

	cid "gx/ipfs/QmeSrf6pzut73u6zLQkRFQ3ygt3k6XFT2kjdYP8Tnkwwyg/go-cid"
)

type DagAPI struct {
	*CoreAPI
	*caopts.DagOptions
}

func (api *DagAPI) Put(ctx context.Context, src io.Reader, opts ...caopts.DagPutOption) ([]coreiface.Node, error) {
	settings, err := caopts.DagPutOptions(opts...)
	if err != nil {
		return nil, err
	}

	codec, ok := cid.CodecToStr[settings.Codec]
	if !ok {
		return nil, fmt.Errorf("invalid codec %d", settings.Codec)
	}

	nds, err := coredag.ParseInputs(settings.InputEnc, codec, src, settings.MhType, settings.MhLength)
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

func (api *DagAPI) Tree(ctx context.Context, p coreiface.Path, opts ...caopts.DagTreeOption) ([]coreiface.Path, error) {
	settings, err := caopts.DagTreeOptions(opts...)
	if err != nil {
		return nil, err
	}

	n, err := api.Get(ctx, p)
	if err != nil {
		return nil, err
	}
	paths := n.Tree("", settings.Depth)
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
	return api.CoreAPI
}
