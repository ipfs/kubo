package routing

import (
	"context"

	"github.com/hashicorp/go-multierror"
	"github.com/ipfs/go-cid"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/multiformats/go-multihash"
)

var _ routinghelpers.ProvideManyRouter = &Composer{}
var _ routing.Routing = &Composer{}

type Composer struct {
	GetValueRouter      routing.Routing
	PutValueRouter      routing.Routing
	FindPeersRouter     routing.Routing
	FindProvidersRouter routing.Routing
	ProvideRouter       routing.Routing
}

func (c *Composer) Provide(ctx context.Context, cid cid.Cid, provide bool) error {
	return c.ProvideRouter.Provide(ctx, cid, provide)
}

func (c *Composer) ProvideMany(ctx context.Context, keys []multihash.Multihash) error {
	pmr, ok := c.ProvideRouter.(routinghelpers.ProvideManyRouter)
	if !ok {
		return nil
	}

	return pmr.ProvideMany(ctx, keys)
}

func (c *Composer) Ready() bool {
	pmr, ok := c.ProvideRouter.(routinghelpers.ProvideManyRouter)
	if !ok {
		return false
	}

	return pmr.Ready()
}

func (c *Composer) FindProvidersAsync(ctx context.Context, cid cid.Cid, count int) <-chan peer.AddrInfo {
	return c.FindProvidersRouter.FindProvidersAsync(ctx, cid, count)
}

func (c *Composer) FindPeer(ctx context.Context, pid peer.ID) (peer.AddrInfo, error) {
	return c.FindPeersRouter.FindPeer(ctx, pid)
}

func (c *Composer) PutValue(ctx context.Context, key string, val []byte, opts ...routing.Option) error {
	return c.PutValueRouter.PutValue(ctx, key, val, opts...)
}

func (c *Composer) GetValue(ctx context.Context, key string, opts ...routing.Option) ([]byte, error) {
	return c.GetValueRouter.GetValue(ctx, key, opts...)
}

func (c *Composer) SearchValue(ctx context.Context, key string, opts ...routing.Option) (<-chan []byte, error) {
	return c.GetValueRouter.SearchValue(ctx, key, opts...)
}

func (c *Composer) Bootstrap(ctx context.Context) error {
	errfp := c.FindPeersRouter.Bootstrap(ctx)
	errfps := c.FindProvidersRouter.Bootstrap(ctx)
	errgv := c.GetValueRouter.Bootstrap(ctx)
	errpv := c.PutValueRouter.Bootstrap(ctx)
	errp := c.ProvideRouter.Bootstrap(ctx)
	return multierror.Append(errfp, errfps, errgv, errpv, errp)
}
