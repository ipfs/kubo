package routing

import (
	"context"

	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
)

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
