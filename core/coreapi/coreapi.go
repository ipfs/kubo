package coreapi

import (
	"context"

	core "github.com/ipfs/go-ipfs/core"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	dag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
	cid "gx/ipfs/QmfSc2xehWmWLnwwYR91Y8QF4xdASypTFVknutoKQS3GHp/go-cid"
)

type UnixfsAPI struct {
	Context context.Context
	Node    *core.IpfsNode
}

func (api *UnixfsAPI) resolve(p string) (*dag.Node, error) {
	pp, err := path.ParsePath(p)
	if err != nil {
		return nil, err
	}

	dagnode, err := core.Resolve(api.Context, api.Node, pp)
	if err == core.ErrNoNamesys {
		return nil, coreiface.ErrOffline
	} else if err != nil {
		return nil, err
	}
	return dagnode, nil
}

func (api *UnixfsAPI) Cat(p string) (coreiface.Reader, error) {
	dagnode, err := api.resolve(p)
	if err != nil {
		return nil, err
	}

	r, err := uio.NewDagReader(api.Context, dagnode, api.Node.DAG)
	if err == uio.ErrIsDir {
		return nil, coreiface.ErrIsDir
	} else if err != nil {
		return nil, err
	}
	return r, nil
}

func (api *UnixfsAPI) Ls(p string) ([]*coreiface.Link, error) {
	dagnode, err := api.resolve(p)
	if err != nil {
		return nil, err
	}

	links := make([]*coreiface.Link, len(dagnode.Links))
	for i, l := range dagnode.Links {
		links[i] = &coreiface.Link{l.Name, l.Size, cid.NewCidV0(l.Hash)}
	}
	return links, nil
}
