package coreapi

import (
	"context"
	"fmt"
	"io"
	"sync"

	gopath "path"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
	coredag "github.com/ipfs/go-ipfs/core/coredag"

	ipld "gx/ipfs/QmX5CsuHyVZeTLxgRSYkgLSDQKb9UjE8xnhQzCEJWWWFsC/go-ipld-format"
	cid "gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
)

type DagAPI CoreAPI

type dagBatch struct {
	api   *DagAPI
	toPut []ipld.Node

	lk sync.Mutex
}

// Put inserts data using specified format and input encoding. Unless used with
// `WithCodes` or `WithHash`, the defaults "dag-cbor" and "sha256" are used.
// Returns the path of the inserted data.
func (api *DagAPI) Put(ctx context.Context, src io.Reader, opts ...caopts.DagPutOption) (coreiface.ResolvedPath, error) {
	nd, err := getNode(src, opts...)

	err = api.node.DAG.Add(ctx, nd)
	if err != nil {
		return nil, err
	}

	return coreiface.IpldPath(nd.Cid()), nil
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
		out[n], err = coreiface.ParsePath(gopath.Join(p.String(), p2))
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}

// Batch creates new DagBatch
func (api *DagAPI) Batch(ctx context.Context) coreiface.DagBatch {
	return &dagBatch{api: api}
}

// Put inserts data using specified format and input encoding. Unless used with
// `WithCodes` or `WithHash`, the defaults "dag-cbor" and "sha256" are used.
// Returns the path of the inserted data.
func (b *dagBatch) Put(ctx context.Context, src io.Reader, opts ...caopts.DagPutOption) (coreiface.ResolvedPath, error) {
	nd, err := getNode(src, opts...)
	if err != nil {
		return nil, err
	}

	b.lk.Lock()
	b.toPut = append(b.toPut, nd)
	b.lk.Unlock()

	return coreiface.IpldPath(nd.Cid()), nil
}

// Commit commits nodes to the datastore and announces them to the network
func (b *dagBatch) Commit(ctx context.Context) error {
	b.lk.Lock()
	defer b.lk.Unlock()
	defer func() {
		b.toPut = nil
	}()

	return b.api.node.DAG.AddMany(ctx, b.toPut)
}

func getNode(src io.Reader, opts ...caopts.DagPutOption) (ipld.Node, error) {
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
	if len(nds) != 1 {
		return nil, fmt.Errorf("got more that one node from ParseInputs")
	}

	return nds[0], nil
}

func (api *DagAPI) core() coreiface.CoreAPI {
	return (*CoreAPI)(api)
}
