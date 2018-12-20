package httpapi

import (
	"context"

	"github.com/ipfs/go-ipfs/core/coreapi/interface"
	"github.com/ipfs/go-ipfs/core/coreapi/interface/options"
)

type NameAPI HttpApi

func (api *NameAPI) Publish(ctx context.Context, p iface.Path, opts ...options.NamePublishOption) (iface.IpnsEntry, error) {
	return nil, ErrNotImplemented
}

func (api *NameAPI) Search(ctx context.Context, name string, opts ...options.NameResolveOption) (<-chan iface.IpnsResult, error) {
	return nil, ErrNotImplemented
}

func (api *NameAPI) Resolve(ctx context.Context, name string, opts ...options.NameResolveOption) (iface.Path, error) {
	// TODO: options!

	req := api.core().request("name/resolve")
	req.Arguments(name)

	var out struct{ Path string }
	if err := req.Exec(ctx, &out); err != nil {
		return nil, err
	}

	return iface.ParsePath(out.Path)
}

func (api *NameAPI) core() *HttpApi {
	return (*HttpApi)(api)
}
