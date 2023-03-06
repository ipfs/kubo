package routing

import (
	"context"

	"github.com/ipfs/go-cid"
	drc "github.com/ipfs/go-delegated-routing/client"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
)

var _ routing.Routing = &reframeRoutingWrapper{}
var _ routinghelpers.ProvideManyRouter = &reframeRoutingWrapper{}

// reframeRoutingWrapper is a wrapper needed to construct the routing.Routing interface from
// delegated-routing library.
type reframeRoutingWrapper struct {
	*drc.Client
	*drc.ContentRoutingClient
}

func (c *reframeRoutingWrapper) Provide(ctx context.Context, id cid.Cid, announce bool) error {
	return c.ContentRoutingClient.Provide(ctx, id, announce)
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

type ProvideManyRouter interface {
	routinghelpers.ProvideManyRouter
	routing.Routing
}

var _ routing.Routing = &httpRoutingWrapper{}
var _ routinghelpers.ProvideManyRouter = &httpRoutingWrapper{}

// httpRoutingWrapper is a wrapper needed to construct the routing.Routing interface from
// http delegated routing.
type httpRoutingWrapper struct {
	routing.ContentRouting
	routinghelpers.ProvideManyRouter
}

func (c *httpRoutingWrapper) Bootstrap(ctx context.Context) error {
	return nil
}

func (c *httpRoutingWrapper) FindPeer(ctx context.Context, id peer.ID) (peer.AddrInfo, error) {
	return peer.AddrInfo{}, routing.ErrNotSupported
}

func (c *httpRoutingWrapper) PutValue(context.Context, string, []byte, ...routing.Option) error {
	return routing.ErrNotSupported
}

func (c *httpRoutingWrapper) GetValue(context.Context, string, ...routing.Option) ([]byte, error) {
	return nil, routing.ErrNotSupported
}

func (c *httpRoutingWrapper) SearchValue(context.Context, string, ...routing.Option) (<-chan []byte, error) {
	out := make(chan []byte)
	close(out)
	return out, routing.ErrNotSupported
}
