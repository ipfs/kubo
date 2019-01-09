package httpapi

import (
	"context"
	"github.com/ipfs/go-cid"
	"io"

	"github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	"github.com/ipfs/go-ipld-format"
)

type ObjectAPI HttpApi

type objectOut struct {
	Hash  string
}

func (api *ObjectAPI) New(ctx context.Context, opts ...caopts.ObjectNewOption) (format.Node, error) {
	options, err := caopts.ObjectNewOptions(opts...)
	if err != nil {
		return nil, err
	}

	var out objectOut
	err = api.core().request("object/new", options.Type).Exec(ctx, &out)
	if err != nil {
		return nil, err
	}

	c, err := cid.Parse(out.Hash)
	if err != nil {
		return nil, err
	}

	return api.core().nodeFromPath(ctx, iface.IpfsPath(c)), nil
}

func (api *ObjectAPI) Put(context.Context, io.Reader, ...caopts.ObjectPutOption) (iface.ResolvedPath, error) {
	panic("implement me")
}

func (api *ObjectAPI) Get(context.Context, iface.Path) (format.Node, error) {
	panic("implement me")
}

func (api *ObjectAPI) Data(context.Context, iface.Path) (io.Reader, error) {
	panic("implement me")
}

func (api *ObjectAPI) Links(context.Context, iface.Path) ([]*format.Link, error) {
	panic("implement me")
}

func (api *ObjectAPI) Stat(context.Context, iface.Path) (*iface.ObjectStat, error) {
	panic("implement me")
}

func (api *ObjectAPI) AddLink(ctx context.Context, base iface.Path, name string, child iface.Path, opts ...caopts.ObjectAddLinkOption) (iface.ResolvedPath, error) {
	panic("implement me")
}

func (api *ObjectAPI) RmLink(ctx context.Context, base iface.Path, link string) (iface.ResolvedPath, error) {
	panic("implement me")
}

func (api *ObjectAPI) AppendData(context.Context, iface.Path, io.Reader) (iface.ResolvedPath, error) {
	panic("implement me")
}

func (api *ObjectAPI) SetData(context.Context, iface.Path, io.Reader) (iface.ResolvedPath, error) {
	panic("implement me")
}

func (api *ObjectAPI) Diff(context.Context, iface.Path, iface.Path) ([]iface.ObjectChange, error) {
	panic("implement me")
}

func (api *ObjectAPI) core() *HttpApi {
	return (*HttpApi)(api)
}
