package rpc

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/ipfs/boxo/path"
	"github.com/ipfs/kubo/core/coreiface/options"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
)

type RoutingAPI HttpApi

func (api *RoutingAPI) Get(ctx context.Context, key string) ([]byte, error) {
	resp, err := api.core().Request("routing/get", key).Send(ctx)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	defer resp.Close()

	var out routing.QueryEvent

	dec := json.NewDecoder(resp.Output)
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}

	res, err := base64.StdEncoding.DecodeString(out.Extra)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (api *RoutingAPI) Put(ctx context.Context, key string, value []byte, opts ...options.RoutingPutOption) error {
	var cfg options.RoutingPutSettings
	for _, o := range opts {
		if err := o(&cfg); err != nil {
			return err
		}
	}

	resp, err := api.core().Request("routing/put", key).
		Option("allow-offline", cfg.AllowOffline).
		FileBody(bytes.NewReader(value)).
		Send(ctx)
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return resp.Error
	}
	return nil
}

func (api *RoutingAPI) FindPeer(ctx context.Context, p peer.ID) (peer.AddrInfo, error) {
	var out struct {
		Type      routing.QueryEventType
		Responses []peer.AddrInfo
	}
	resp, err := api.core().Request("routing/findpeer", p.String()).Send(ctx)
	if err != nil {
		return peer.AddrInfo{}, err
	}
	if resp.Error != nil {
		return peer.AddrInfo{}, resp.Error
	}
	defer resp.Close()
	dec := json.NewDecoder(resp.Output)
	for {
		if err := dec.Decode(&out); err != nil {
			return peer.AddrInfo{}, err
		}
		if out.Type == routing.FinalPeer {
			return out.Responses[0], nil
		}
	}
}

func (api *RoutingAPI) FindProviders(ctx context.Context, p path.Path, opts ...options.RoutingFindProvidersOption) (<-chan peer.AddrInfo, error) {
	options, err := options.RoutingFindProvidersOptions(opts...)
	if err != nil {
		return nil, err
	}

	rp, _, err := api.core().ResolvePath(ctx, p)
	if err != nil {
		return nil, err
	}

	resp, err := api.core().Request("routing/findprovs", rp.RootCid().String()).
		Option("num-providers", options.NumProviders).
		Send(ctx)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	res := make(chan peer.AddrInfo)

	go func() {
		defer resp.Close()
		defer close(res)
		dec := json.NewDecoder(resp.Output)

		for {
			var out struct {
				Extra     string
				Type      routing.QueryEventType
				Responses []peer.AddrInfo
			}

			if err := dec.Decode(&out); err != nil {
				return // todo: handle this somehow
			}
			if out.Type == routing.QueryError {
				return // usually a 'not found' error
				// todo: handle other errors
			}
			if out.Type == routing.Provider {
				for _, pi := range out.Responses {
					select {
					case res <- pi:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return res, nil
}

func (api *RoutingAPI) Provide(ctx context.Context, p path.Path, opts ...options.RoutingProvideOption) error {
	options, err := options.RoutingProvideOptions(opts...)
	if err != nil {
		return err
	}

	rp, _, err := api.core().ResolvePath(ctx, p)
	if err != nil {
		return err
	}

	return api.core().Request("routing/provide", rp.RootCid().String()).
		Option("recursive", options.Recursive).
		Exec(ctx, nil)
}

func (api *RoutingAPI) core() *HttpApi {
	return (*HttpApi)(api)
}
