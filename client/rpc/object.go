package rpc

import (
	"context"

	"github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	iface "github.com/ipfs/kubo/core/coreiface"
	caopts "github.com/ipfs/kubo/core/coreiface/options"
)

type ObjectAPI HttpApi

type objectOut struct {
	Hash string
}

func (api *ObjectAPI) AddLink(ctx context.Context, base path.Path, name string, child path.Path, opts ...caopts.ObjectAddLinkOption) (path.ImmutablePath, error) {
	options, err := caopts.ObjectAddLinkOptions(opts...)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	var out objectOut
	err = api.core().Request("object/patch/add-link", base.String(), name, child.String()).
		Option("create", options.Create).
		Exec(ctx, &out)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	c, err := cid.Parse(out.Hash)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	return path.FromCid(c), nil
}

func (api *ObjectAPI) RmLink(ctx context.Context, base path.Path, link string) (path.ImmutablePath, error) {
	var out objectOut
	err := api.core().Request("object/patch/rm-link", base.String(), link).
		Exec(ctx, &out)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	c, err := cid.Parse(out.Hash)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	return path.FromCid(c), nil
}

type change struct {
	Type   iface.ChangeType
	Path   string
	Before cid.Cid
	After  cid.Cid
}

func (api *ObjectAPI) Diff(ctx context.Context, a path.Path, b path.Path) ([]iface.ObjectChange, error) {
	var out struct {
		Changes []change
	}
	if err := api.core().Request("object/diff", a.String(), b.String()).Exec(ctx, &out); err != nil {
		return nil, err
	}
	res := make([]iface.ObjectChange, len(out.Changes))
	for i, ch := range out.Changes {
		res[i] = iface.ObjectChange{
			Type: ch.Type,
			Path: ch.Path,
		}
		if ch.Before != cid.Undef {
			res[i].Before = path.FromCid(ch.Before)
		}
		if ch.After != cid.Undef {
			res[i].After = path.FromCid(ch.After)
		}
	}
	return res, nil
}

func (api *ObjectAPI) core() *HttpApi {
	return (*HttpApi)(api)
}
