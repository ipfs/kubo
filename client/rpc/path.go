package rpc

import (
	"context"

	"github.com/ipfs/boxo/path"
	cid "github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
)

func (api *HttpApi) ResolvePath(ctx context.Context, p path.Path) (path.ImmutablePath, []string, error) {
	var out struct {
		Cid     cid.Cid
		RemPath string
	}

	var err error
	if p.Namespace() == path.IPNSNamespace {
		if p, err = api.Name().Resolve(ctx, p.String()); err != nil {
			return path.ImmutablePath{}, nil, err
		}
	}

	if err := api.Request("dag/resolve", p.String()).Exec(ctx, &out); err != nil {
		return path.ImmutablePath{}, nil, err
	}

	p, err = path.NewPathFromSegments(p.Namespace(), out.Cid.String(), out.RemPath)
	if err != nil {
		return path.ImmutablePath{}, nil, err
	}

	imPath, err := path.NewImmutablePath(p)
	if err != nil {
		return path.ImmutablePath{}, nil, err
	}

	return imPath, path.StringToSegments(out.RemPath), nil
}

func (api *HttpApi) ResolveNode(ctx context.Context, p path.Path) (ipld.Node, error) {
	rp, _, err := api.ResolvePath(ctx, p)
	if err != nil {
		return nil, err
	}

	return api.Dag().Get(ctx, rp.RootCid())
}
