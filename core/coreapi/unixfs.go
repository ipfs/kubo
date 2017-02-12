package coreapi

import (
	"context"
	"io"

	core "github.com/ipfs/go-ipfs/core"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	coreunix "github.com/ipfs/go-ipfs/core/coreunix"
	uio "github.com/ipfs/go-ipfs/unixfs/io"

	cid "gx/ipfs/QmcTcsTvfaeEBRFo1TkFgT8sRmgi1n1LTZpecfVP8fzpGD/go-cid"
)

type UnixfsAPI struct {
	node *core.IpfsNode
}

func NewUnixfsAPI(n *core.IpfsNode) coreiface.UnixfsAPI {
	api := &UnixfsAPI{n}
	return api
}

func (api *UnixfsAPI) Add(ctx context.Context, r io.Reader) (*cid.Cid, error) {
	k, err := coreunix.AddWithContext(ctx, api.node, r)
	if err != nil {
		return nil, err
	}
	return cid.Decode(k)
}

func (api *UnixfsAPI) Cat(ctx context.Context, p string) (coreiface.Reader, error) {
	dagnode, err := resolve(ctx, api.node, p)
	if err != nil {
		return nil, err
	}

	r, err := uio.NewDagReader(ctx, dagnode, api.node.DAG)
	if err == uio.ErrIsDir {
		return nil, coreiface.ErrIsDir
	} else if err != nil {
		return nil, err
	}
	return r, nil
}

func (api *UnixfsAPI) Ls(ctx context.Context, p string) ([]*coreiface.Link, error) {
	dagnode, err := resolve(ctx, api.node, p)
	if err != nil {
		return nil, err
	}

	l := dagnode.Links()
	links := make([]*coreiface.Link, len(l))
	for i, l := range l {
		links[i] = &coreiface.Link{l.Name, l.Size, l.Cid}
	}
	return links, nil
}
