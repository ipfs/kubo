package coreapi

import (
	"context"
	"fmt"
	"io"

	gopath "path"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
	coredag "github.com/ipfs/go-ipfs/core/coredag"

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

type DagAPI CoreAPI

// Put inserts data using specified format and input encoding. Unless used with
// `WithCodes` or `WithHash`, the defaults "dag-cbor" and "sha256" are used.
// Returns the path of the inserted data.
func (api *DagAPI) Put(ctx context.Context, src io.Reader, opts ...caopts.DagPutOption) (coreiface.Path, error) {
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

	err = api.node.DAG.Add(ctx, nds[0])
	if err != nil {
		return nil, err
	}

	return ParseCid(nds[0].Cid()), nil
}

// Get resolves `path` using Unixfs resolver, returns the resolved Node.
func (api *DagAPI) Get(ctx context.Context, path coreiface.Path) (ipld.Node, error) {
	return api.core().ResolveNode(ctx, path)
}

// Tree returns list of paths within a node specified by the path `p`.
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
	return (*CoreAPI)(api)
}
