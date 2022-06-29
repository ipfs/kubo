package routing

import (
	"context"
	"sync"

	"github.com/hashicorp/go-multierror"
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

var _ TieredContentRouter = &ContentRoutingWrapper{}

type ContentRoutingWrapper struct {
	ContentRoutings []routing.ContentRouting
}

// Provide adds the given cid to the content routing system. If 'true' is
// passed, it also announces it, otherwise it is just kept in the local
// accounting of which objects are being provided.
func (crw *ContentRoutingWrapper) Provide(ctx context.Context, cid cid.Cid, announce bool) error {
	c := len(crw.ContentRoutings)
	wg := sync.WaitGroup{}
	wg.Add(c)

	errors := make([]error, c)

	for i, cr := range crw.ContentRoutings {
		go func(cr routing.ContentRouting, i int) {
			errors[i] = cr.Provide(ctx, cid, announce)
			wg.Done()
		}(cr, i)
	}

	wg.Wait()

	var out []error
	success := false
	for _, e := range errors {
		switch e {
		case nil:
			success = true
		case routing.ErrNotSupported:
		default:
			out = append(out, e)
		}
	}
	switch len(out) {
	case 0:
		if success {
			// No errors and at least one router succeeded.
			return nil
		}
		// No routers supported this operation.
		return routing.ErrNotSupported
	case 1:
		return out[0]
	default:
		return &multierror.Error{Errors: out}
	}
}

// Search for peers who are able to provide a given key
//
// When count is 0, this method will return an unbounded number of
// results.
func (crw *ContentRoutingWrapper) FindProvidersAsync(ctx context.Context, cid cid.Cid, count int) <-chan peer.AddrInfo {
	subCtx, cancel := context.WithCancel(ctx)

	aich := make(chan peer.AddrInfo)

	for _, ri := range crw.ContentRoutings {
		fpch := ri.FindProvidersAsync(subCtx, cid, count)
		go func() {
			for ai := range fpch {
				aich <- ai
			}
		}()
	}

	out := make(chan peer.AddrInfo)

	go func() {
		defer cancel()
		c := 0
		doCount := true
		if count <= 0 {
			doCount = false
		}

		for ai := range aich {
			if c >= count && doCount {
				return
			}

			out <- ai

			if doCount {
				c++
			}
		}
	}()

	return out
}
