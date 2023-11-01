package coreapi

import (
	"context"
	"errors"
	"strings"

	coreiface "github.com/ipfs/boxo/coreiface"
	caopts "github.com/ipfs/boxo/coreiface/options"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

type RoutingAPI CoreAPI

func (r *RoutingAPI) Get(ctx context.Context, key string) ([]byte, error) {
	if !r.nd.IsOnline {
		return nil, coreiface.ErrOffline
	}

	dhtKey, err := normalizeKey(key)
	if err != nil {
		return nil, err
	}

	return r.routing.GetValue(ctx, dhtKey)
}

func (r *RoutingAPI) Put(ctx context.Context, key string, value []byte, opts ...caopts.RoutingPutOption) error {
	options, err := caopts.RoutingPutOptions(opts...)
	if err != nil {
		return err
	}

	err = r.checkOnline(options.AllowOffline)
	if err != nil {
		return err
	}

	dhtKey, err := normalizeKey(key)
	if err != nil {
		return err
	}

	return r.routing.PutValue(ctx, dhtKey, value)
}

func normalizeKey(s string) (string, error) {
	parts := strings.Split(s, "/")
	if len(parts) != 3 ||
		parts[0] != "" ||
		!(parts[1] == "ipns" || parts[1] == "pk") {
		return "", errors.New("invalid key")
	}

	k, err := peer.Decode(parts[2])
	if err != nil {
		return "", err
	}
	return strings.Join(append(parts[:2], string(k)), "/"), nil
}
