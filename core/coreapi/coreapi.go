package coreapi

import (
	"context"

	core "github.com/ipfs/go-ipfs/core"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	ipfspath "github.com/ipfs/go-ipfs/path"
	uio "github.com/ipfs/go-ipfs/unixfs/io"

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

type CoreAPI struct {
	node *core.IpfsNode
}

// NewCoreAPI creates new instance of IPFS CoreAPI backed by go-ipfs Node.
func NewCoreAPI(n *core.IpfsNode) coreiface.CoreAPI {
	api := &CoreAPI{n}
	return api
}

// Unixfs returns the UnixfsAPI interface backed by the go-ipfs node
func (api *CoreAPI) Unixfs() coreiface.UnixfsAPI {
	return (*UnixfsAPI)(api)
}

// Dag returns the DagAPI interface backed by the go-ipfs node
func (api *CoreAPI) Dag() coreiface.DagAPI {
	return &DagAPI{api, nil}
}

// Name returns the NameAPI interface backed by the go-ipfs node
func (api *CoreAPI) Name() coreiface.NameAPI {
	return &NameAPI{api, nil}
}

// Key returns the KeyAPI interface backed by the go-ipfs node
func (api *CoreAPI) Key() coreiface.KeyAPI {
	return &KeyAPI{api, nil}
}

//Object returns the ObjectAPI interface backed by the go-ipfs node
func (api *CoreAPI) Object() coreiface.ObjectAPI {
	return &ObjectAPI{api, nil}
}

// ResolveNode resolves the path `p` using Unixfx resolver, gets and returns the
// resolved Node.
func (api *CoreAPI) ResolveNode(ctx context.Context, p coreiface.Path) (coreiface.Node, error) {
	p, err := api.ResolvePath(ctx, p)
	if err != nil {
		return nil, err
	}

	node, err := api.node.DAG.Get(ctx, p.Cid())
	if err != nil {
		return nil, err
	}
	return node, nil
}

// ResolvePath resolves the path `p` using Unixfs resolver, returns the
// resolved path.
// TODO: store all of ipfspath.Resolver.ResolvePathComponents() in Path
func (api *CoreAPI) ResolvePath(ctx context.Context, p coreiface.Path) (coreiface.Path, error) {
	if p.Resolved() {
		return p, nil
	}

	r := &ipfspath.Resolver{
		DAG:         api.node.DAG,
		ResolveOnce: uio.ResolveUnixfsOnce,
	}

	p2 := ipfspath.FromString(p.String())
	node, err := core.Resolve(ctx, api.node.Namesys, r, p2)
	if err == core.ErrNoNamesys {
		return nil, coreiface.ErrOffline
	} else if err != nil {
		return nil, err
	}

	var root *cid.Cid
	if p2.IsJustAKey() {
		root = node.Cid()
	}

	return ResolvedPath(p.String(), node.Cid(), root), nil
}

// Implements coreiface.Path
type path struct {
	path ipfspath.Path
	cid  *cid.Cid
	root *cid.Cid
}

// ParsePath parses path `p` using ipfspath parser, returns the parsed path.
func ParsePath(p string) (coreiface.Path, error) {
	pp, err := ipfspath.ParsePath(p)
	if err != nil {
		return nil, err
	}
	return &path{path: pp}, nil
}

// ParseCid parses the path from `c`, retruns the parsed path.
func ParseCid(c *cid.Cid) coreiface.Path {
	return &path{path: ipfspath.FromCid(c), cid: c, root: c}
}

// ResolvePath parses path from string `p`, returns parsed path.
func ResolvedPath(p string, c *cid.Cid, r *cid.Cid) coreiface.Path {
	return &path{path: ipfspath.FromString(p), cid: c, root: r}
}

func (p *path) String() string { return p.path.String() }
func (p *path) Cid() *cid.Cid  { return p.cid }
func (p *path) Root() *cid.Cid { return p.root }
func (p *path) Resolved() bool { return p.cid != nil }
