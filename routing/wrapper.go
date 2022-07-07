package routing

import (
	"context"

	"github.com/ipfs/go-cid"
	drc "github.com/ipfs/go-delegated-routing/client"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	"github.com/multiformats/go-multihash"
	"golang.org/x/sync/errgroup"
)

var _ routing.Routing = &reframeRoutingWrapper{}

// reframeRoutingWrapper is a wrapper needed to construct the routing.Routing interface from
// delegated-routing library.
type reframeRoutingWrapper struct {
	*drc.Client
	*drc.ContentRoutingClient
}

func (c *reframeRoutingWrapper) FindProvidersAsync(ctx context.Context, cid cid.Cid, count int) <-chan peer.AddrInfo {
	return c.ContentRoutingClient.FindProvidersAsync(ctx, cid, count)
}

func (c *reframeRoutingWrapper) Bootstrap(ctx context.Context) error {
	return nil
}

func (c *reframeRoutingWrapper) FindPeer(ctx context.Context, id peer.ID) (peer.AddrInfo, error) {
	return peer.AddrInfo{}, routing.ErrNotSupported
}

type ProvideMany interface {
	ProvideMany(ctx context.Context, keys []multihash.Multihash) error
	Ready() bool
}

var _ ProvideMany = &ProvideManyWrapper{}

type ProvideManyWrapper struct {
	pms []ProvideMany
}

func (pmw *ProvideManyWrapper) ProvideMany(ctx context.Context, keys []multihash.Multihash) error {
	var g errgroup.Group
	for _, pm := range pmw.pms {
		pm := pm
		g.Go(func() error {
			return pm.ProvideMany(ctx, keys)
		})
	}

	return g.Wait()
}

// Ready is ready if all providers are ready
func (pmw *ProvideManyWrapper) Ready() bool {
	out := true
	for _, pm := range pmw.pms {
		out = out && pm.Ready()
	}

	return out
}
