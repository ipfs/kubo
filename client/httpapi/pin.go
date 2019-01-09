package httpapi

import (
	"context"
	"github.com/ipfs/go-cid"

	"github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
)

type PinAPI HttpApi

type pinRefKeyObject struct {
	Type string
}

type pinRefKeyList struct {
	Keys map[string]pinRefKeyObject
}

type pin struct {
	path iface.ResolvedPath
	typ  string
}

func (p *pin) Path() iface.ResolvedPath {
	return p.path
}

func (p *pin) Type() string {
	return p.typ
}

func (api *PinAPI) Add(context.Context, iface.Path, ...caopts.PinAddOption) error {
	panic("implement me")
}

func (api *PinAPI) Ls(ctx context.Context, opts ...caopts.PinLsOption) ([]iface.Pin, error) {
	options, err := caopts.PinLsOptions(opts...)
	if err != nil {
		return nil, err
	}

	var out pinRefKeyList
	err = api.core().request("pin/ls").
		Option("type", options.Type).Exec(ctx, &out)
	if err != nil {
		return nil, err
	}

	pins := make([]iface.Pin, 0, len(out.Keys))
	for hash, p := range out.Keys {
		c, err := cid.Parse(hash)
		if err != nil {
			return nil, err
		}
		pins = append(pins, &pin{typ: p.Type, path: iface.IpldPath(c)})
	}

	return pins, nil
}

func (api *PinAPI) Rm(ctx context.Context, p iface.Path) error {
	return api.core().request("pin/rm", p.String()).Exec(ctx, nil)
}

func (api *PinAPI) Update(ctx context.Context, from iface.Path, to iface.Path, opts ...caopts.PinUpdateOption) error {
	panic("implement me")
}

func (api *PinAPI) Verify(context.Context) (<-chan iface.PinStatus, error) {
	panic("implement me")
}

func (api *PinAPI) core() *HttpApi {
	return (*HttpApi)(api)
}
