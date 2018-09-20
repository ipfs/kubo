package coreapi

import (
	"context"
	"errors"
	"fmt"
	"io"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	"github.com/ipfs/go-ipfs/core/coreapi/interface/options"
	"github.com/ipfs/go-ipfs/core/coreunix"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	cidutil "gx/ipfs/QmQJSeE3CX4zos9qeaG8EhecEK9zvrTEfTG84J8C5NVRwt/go-cidutil"
	"gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit/files"
	uio "gx/ipfs/QmU4x3742bvgfxJsByEDpBnifJqjJdV6x528co4hwKCn46/go-unixfs/io"
	dag "gx/ipfs/QmcBoNcAP6qDjgRBew7yjvCqHq7p5jMstE44jPUBWBxzsV/go-merkledag"
	ipld "gx/ipfs/QmdDXJs4axxefSPgK6Y1QhpJWKuDPnGJiqgq4uncb4rFHL/go-ipld-format"
)

type UnixfsAPI CoreAPI

// Add builds a merkledag node from a reader, adds it to the blockstore,
// and returns the key representing that node.
func (api *UnixfsAPI) Add(ctx context.Context, r io.ReadCloser, opts ...options.UnixfsAddOption) (coreiface.ResolvedPath, error) {
	settings, err := options.UnixfsAddOptions(opts...)
	if err != nil {
		return nil, err
	}

	// TODO: move to options
	// (hash != "sha2-256") -> CIDv1
	if settings.MhType != mh.SHA2_256 {
		switch settings.CidVersion {
		case 0:
			return nil, errors.New("CIDv0 only supports sha2-256")
		case 1, -1:
			settings.CidVersion = 1
		default:
			return nil, fmt.Errorf("unknown CID version: %d", settings.CidVersion)
		}
	} else {
		if settings.CidVersion < 0 {
			// Default to CIDv0
			settings.CidVersion = 0
		}
	}

	// cidV1 -> raw blocks (by default)
	if settings.CidVersion > 0 && !settings.RawLeavesSet {
		settings.RawLeaves = true
	}

	prefix, err := dag.PrefixForCidVersion(settings.CidVersion)
	if err != nil {
		return nil, err
	}

	prefix.MhType = settings.MhType
	prefix.MhLength = -1

	outChan := make(chan interface{}, 1)

	fileAdder, err := coreunix.NewAdder(ctx, api.node.Pinning, api.node.Blockstore, api.node.DAG)
	if err != nil {
		return nil, err
	}

	fileAdder.Out = outChan
	fileAdder.Chunker = settings.Chunker
	//fileAdder.Progress = progress
	//fileAdder.Hidden = hidden
	//fileAdder.Wrap = wrap
	//fileAdder.Pin = dopin
	fileAdder.Silent = false
	fileAdder.RawLeaves = settings.RawLeaves
	//fileAdder.NoCopy = nocopy
	//fileAdder.Name = pathName
	fileAdder.CidBuilder = prefix

	switch settings.Layout {
	case options.BalancedLayout:
		// Default
	case options.TrickleLeyout:
		fileAdder.Trickle = true
	default:
		return nil, fmt.Errorf("unknown layout: %d", settings.Layout)
	}

	if settings.InlineLimit > 0 {
		fileAdder.CidBuilder = cidutil.InlineBuilder{
			Builder: fileAdder.CidBuilder,
			Limit:   settings.InlineLimit,
		}
	}

	err = fileAdder.AddFile(files.NewReaderFile("", "", r, nil))
	if err != nil {
		return nil, err
	}

	if _, err = fileAdder.Finalize(); err != nil {
		return nil, err
	}

	for {
		select {
		case r := <-outChan:
			output := r.(*coreunix.AddedObject)
			if output.Hash != "" {
				c, err := cid.Parse(output.Hash)
				if err != nil {
					return nil, err
				}

				return coreiface.IpfsPath(c), err
			}
		}
	}
}

// Cat returns the data contained by an IPFS or IPNS object(s) at path `p`.
func (api *UnixfsAPI) Cat(ctx context.Context, p coreiface.Path) (coreiface.Reader, error) {
	dget := api.node.DAG // TODO: use a session here once routing perf issues are resolved

	dagnode, err := api.core().ResolveNode(ctx, p)
	if err != nil {
		return nil, err
	}

	r, err := uio.NewDagReader(ctx, dagnode, dget)
	if err == uio.ErrIsDir {
		return nil, coreiface.ErrIsDir
	} else if err != nil {
		return nil, err
	}
	return r, nil
}

// Ls returns the contents of an IPFS or IPNS object(s) at path p, with the format:
// `<link base58 hash> <link size in bytes> <link name>`
func (api *UnixfsAPI) Ls(ctx context.Context, p coreiface.Path) ([]*ipld.Link, error) {
	dagnode, err := api.core().ResolveNode(ctx, p)
	if err != nil {
		return nil, err
	}

	var ndlinks []*ipld.Link
	dir, err := uio.NewDirectoryFromNode(api.node.DAG, dagnode)
	switch err {
	case nil:
		l, err := dir.Links(ctx)
		if err != nil {
			return nil, err
		}
		ndlinks = l
	case uio.ErrNotADir:
		ndlinks = dagnode.Links()
	default:
		return nil, err
	}

	links := make([]*ipld.Link, len(ndlinks))
	for i, l := range ndlinks {
		links[i] = &ipld.Link{Name: l.Name, Size: l.Size, Cid: l.Cid}
	}
	return links, nil
}

func (api *UnixfsAPI) core() coreiface.CoreAPI {
	return (*CoreAPI)(api)
}
