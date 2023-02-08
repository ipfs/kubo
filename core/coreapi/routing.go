package coreapi

import (
	"context"
	"fmt"

	"github.com/ipfs/go-path"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

type RoutingAPI CoreAPI

func (r *RoutingAPI) Get(ctx context.Context, p ipath.Path) ([]byte, error) {
	if !r.nd.IsOnline {
		return nil, coreiface.ErrOffline
	}

	dhtKey, err := normalizeKey(p)
	if err != nil {
		return nil, err
	}

	return r.routing.GetValue(ctx, dhtKey)
}

func (r *RoutingAPI) Put(ctx context.Context, p ipath.Path, value []byte) error {
	if !r.nd.IsOnline {
		return coreiface.ErrOffline
	}

	dhtKey, err := normalizeKey(p)
	if err != nil {
		return err
	}

	return r.routing.PutValue(ctx, dhtKey, value)
}

func normalizeKey(p ipath.Path) (string, error) {
	if err := p.IsValid(); err != nil {
		return "", fmt.Errorf("invalid key: %w", err)
	}

	ns := p.Namespace()
	if ns != "ipns" && ns != "pk" {
		return "", fmt.Errorf("key has unexpected namespace: %s", ns)
	}

	parts := path.SplitList(p.String())
	if len(parts) != 3 {
		return "", fmt.Errorf("key has unexpected number of parts: %d, expected 3", len(parts))
	}

	k, err := peer.Decode(parts[2])
	if err != nil {
		return "", err
	}
	return path.Join(append(parts[:2], string(k))), nil
}
