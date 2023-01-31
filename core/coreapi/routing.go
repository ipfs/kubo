package coreapi

import (
	"context"
	"errors"

	"github.com/ipfs/go-path"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
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

func (r *RoutingAPI) Put(ctx context.Context, key string, value []byte) error {
	if !r.nd.IsOnline {
		return coreiface.ErrOffline
	}

	dhtKey, err := normalizeKey(key)
	if err != nil {
		return err
	}

	return r.routing.PutValue(ctx, dhtKey, value)
}

func normalizeKey(s string) (string, error) {
	parts := path.SplitList(s)
	if len(parts) != 3 ||
		parts[0] != "" ||
		!(parts[1] == "ipns" || parts[1] == "pk") {
		return "", errors.New("invalid key")
	}

	k, err := peer.Decode(parts[2])
	if err != nil {
		return "", err
	}
	return path.Join(append(parts[:2], string(k))), nil
}
