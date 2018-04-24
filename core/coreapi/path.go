package coreapi

import (
	context "context"
	fmt "fmt"
	gopath "path"

	core "github.com/ipfs/go-ipfs/core"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	namesys "github.com/ipfs/go-ipfs/namesys"
	ipfspath "github.com/ipfs/go-ipfs/path"
	resolver "github.com/ipfs/go-ipfs/path/resolver"
	uio "github.com/ipfs/go-ipfs/unixfs/io"

	ipld "gx/ipfs/QmWi2BYBL5gJ3CiAiQchg6rn1A8iBsrWy51EYxvHVjFvLb/go-ipld-format"
	cid "gx/ipfs/QmapdYm1b22Frv3k17fqrBYTFRxwiaVJkB299Mfn33edeB/go-cid"
)

// path implements coreiface.Path
type path struct {
	path ipfspath.Path
}

// resolvedPath implements coreiface.resolvedPath
type resolvedPath struct {
	path
	cid       *cid.Cid
	root      *cid.Cid
	remainder string
}

// IpfsPath parses the path from `c`, reruns the parsed path.
func (api *CoreAPI) IpfsPath(c *cid.Cid) coreiface.ResolvedPath {
	return &resolvedPath{
		path:      path{ipfspath.Path("/ipfs/" + c.String())},
		cid:       c,
		root:      c,
		remainder: "",
	}
}

// IpldPath parses the path from `c`, reruns the parsed path.
func (api *CoreAPI) IpldPath(c *cid.Cid) coreiface.ResolvedPath {
	return &resolvedPath{
		path:      path{ipfspath.Path("/ipld/" + c.String())},
		cid:       c,
		root:      c,
		remainder: "",
	}
}

// ResolveNode resolves the path `p` using Unixfs resolver, gets and returns the
// resolved Node.
func (api *CoreAPI) ResolveNode(ctx context.Context, p coreiface.Path) (ipld.Node, error) {
	return resolveNode(ctx, api.node.DAG, api.node.Namesys, p)
}

// ResolvePath resolves the path `p` using Unixfs resolver, returns the
// resolved path.
func (api *CoreAPI) ResolvePath(ctx context.Context, p coreiface.Path) (coreiface.ResolvedPath, error) {
	return resolvePath(ctx, api.node.DAG, api.node.Namesys, p)
}

func resolveNode(ctx context.Context, ng ipld.NodeGetter, nsys namesys.NameSystem, p coreiface.Path) (ipld.Node, error) {
	rp, err := resolvePath(ctx, ng, nsys, p)
	if err != nil {
		return nil, err
	}

	node, err := ng.Get(ctx, rp.Cid())
	if err != nil {
		return nil, err
	}
	return node, nil
}

func resolvePath(ctx context.Context, ng ipld.NodeGetter, nsys namesys.NameSystem, p coreiface.Path) (coreiface.ResolvedPath, error) {
	if _, ok := p.(coreiface.ResolvedPath); ok {
		return p.(coreiface.ResolvedPath), nil
	}

	ipath := p.(*path).path
	ipath, err := core.ResolveIPNS(ctx, nsys, ipath)
	if err == core.ErrNoNamesys {
		return nil, coreiface.ErrOffline
	} else if err != nil {
		return nil, err
	}

	var resolveOnce resolver.ResolveOnce

	switch ipath.Segments()[0] {
	case "ipfs":
		resolveOnce = uio.ResolveUnixfsOnce
	case "ipld":
		resolveOnce = resolver.ResolveSingle
	default:
		return nil, fmt.Errorf("unsupported path namespace: %s", p.Namespace())
	}

	r := &resolver.Resolver{
		DAG:         ng,
		ResolveOnce: resolveOnce,
	}

	node, rest, err := r.ResolveToLastNode(ctx, ipath)
	if err != nil {
		return nil, err
	}

	root, err := cid.Parse(ipath.Segments()[1])
	if err != nil {
		return nil, err
	}

	return &resolvedPath{
		path:      path{ipath},
		cid:       node.Cid(),
		root:      root,
		remainder: gopath.Join(rest...),
	}, nil
}

// ParsePath parses path `p` using ipfspath parser, returns the parsed path.
func (api *CoreAPI) ParsePath(p string) (coreiface.Path, error) {
	pp, err := ipfspath.ParsePath(p)
	if err != nil {
		return nil, err
	}

	return &path{path: pp}, nil
}

func (p *path) String() string {
	return p.path.String()
}

func (p *path) Namespace() string {
	if len(p.path.Segments()) < 1 {
		panic("path without namespace") //this shouldn't happen under any scenario
	}
	return p.path.Segments()[0]
}

func (p *path) Mutable() bool {
	//TODO: MFS: check for /local
	return p.Namespace() == "ipns"
}

func (p *resolvedPath) Cid() *cid.Cid {
	return p.cid
}

func (p *resolvedPath) Root() *cid.Cid {
	return p.root
}

func (p *resolvedPath) Remainder() string {
	return p.remainder
}
