package routing

import (
	"context"

	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/core/routing"
)

type ProvideManyRouter interface {
	routinghelpers.ProvideManyRouter
	routing.Routing
}

var (
	_ routing.Routing                  = &httpRoutingWrapper{}
	_ routinghelpers.ProvideManyRouter = &httpRoutingWrapper{}
)

// httpRoutingWrapper is a wrapper needed to construct the routing.Routing interface from
// http delegated routing.
type httpRoutingWrapper struct {
	routing.ContentRouting
	routing.PeerRouting
	routing.ValueStore
	routinghelpers.ProvideManyRouter
}

func (c *httpRoutingWrapper) Bootstrap(ctx context.Context) error {
	return nil
}
