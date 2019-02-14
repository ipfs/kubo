package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/ipfs/interface-go-ipfs-core"
	caopts "github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/options/namesys"
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
		Option("lifetime", options.ValidTime).
		Option("resolve", false)

	if options.TTL != nil {
		req.Option("ttl", options.TTL)
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
		Option("dht-timeout", ropts.DhtTimeout).
		Option("stream", true)
	resp, err := req.Send(ctx)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}

	res := make(chan iface.IpnsResult)

	go func() {
		defer close(res)
		defer resp.Close()

		dec := json.NewDecoder(resp.Output)

		for {
			var out struct{ Path string }
			err := dec.Decode(&out)
			if err == io.EOF {
				return
			}
			var ires iface.IpnsResult
			if err == nil {
				ires.Path, err = iface.ParsePath(out.Path)
			}

			select {
			case res <- ires:
			case <-ctx.Done():
			}
			if err != nil {
				return
			}
		}
	}()

	return res, nil
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
		Option("dht-timeout", ropts.DhtTimeout)

	var out struct{ Path string }
	if err := req.Exec(ctx, &out); err != nil {
		return nil, err
	}

	return iface.ParsePath(out.Path)
}

func (api *NameAPI) core() *HttpApi {
	return (*HttpApi)(api)
}
