package rpc

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/ipfs/boxo/coreiface/options"
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

func (api *RoutingAPI) core() *HttpApi {
	return (*HttpApi)(api)
}
