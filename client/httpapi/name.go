package httpapi

import (
	"context"
	"fmt"
	"github.com/ipfs/go-ipfs/namesys/opts"

	"github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
)

type NameAPI HttpApi

type ipnsEntry struct {
	JName  string `json:"Name"`
	JValue string `json:"Value"`
}

func (e *ipnsEntry) valid() (iface.Path, error) {
	return iface.ParsePath(e.JValue)
}

func (e *ipnsEntry) Name() string {
	return e.JName
}

func (e *ipnsEntry) Value() iface.Path {
	p, _ := e.valid()
	return p
}

func (api *NameAPI) Publish(ctx context.Context, p iface.Path, opts ...caopts.NamePublishOption) (iface.IpnsEntry, error) {
	options, err := caopts.NamePublishOptions(opts...)
	if err != nil {
		return nil, err
	}

	req := api.core().request("name/publish", p.String()).
		Option("key", options.Key).
		Option("allow-offline", options.AllowOffline).
		Option("lifetime", options.ValidTime.String()).
		Option("resolve", false)

	if options.TTL != nil {
		req.Option("ttl", options.TTL.String())
	}

	var out ipnsEntry
	if err := req.Exec(ctx, &out); err != nil {
		return nil, err
	}
	if _, err := out.valid(); err != nil {
		return nil, err
	}

	return &out, nil
}

func (api *NameAPI) Search(ctx context.Context, name string, opts ...caopts.NameResolveOption) (<-chan iface.IpnsResult, error) {
	return nil, ErrNotImplemented
}

func (api *NameAPI) Resolve(ctx context.Context, name string, opts ...caopts.NameResolveOption) (iface.Path, error) {
	options, err := caopts.NameResolveOptions(opts...)
	if err != nil {
		return nil, err
	}

	ropts := nsopts.ProcessOpts(options.ResolveOpts)
	if ropts.Depth != nsopts.DefaultDepthLimit && ropts.Depth != 1 {
		return nil, fmt.Errorf("Name.Resolve: depth other than 1 or %d not supported", nsopts.DefaultDepthLimit)
	}

	req := api.core().request("name/resolve", name).
		Option("nocache", !options.Cache).
		Option("recursive", ropts.Depth != 1).
		Option("dht-record-count", ropts.DhtRecordCount).
		Option("dht-timeout", ropts.DhtTimeout.String())

	var out struct{ Path string }
	if err := req.Exec(ctx, &out); err != nil {
		return nil, err
	}

	return iface.ParsePath(out.Path)
}

func (api *NameAPI) core() *HttpApi {
	return (*HttpApi)(api)
}
