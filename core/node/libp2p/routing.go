package libp2p

import (
	"context"
	"fmt"
	"runtime/debug"
	"sort"
	"time"

	"github.com/cenkalti/backoff/v4"
	ds "github.com/ipfs/go-datastore"
	offroute "github.com/ipfs/go-ipfs-routing/offline"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	namesys "github.com/libp2p/go-libp2p-pubsub-router"
	record "github.com/libp2p/go-libp2p-record"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"go.uber.org/fx"

	config "github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/node/helpers"
	irouting "github.com/ipfs/kubo/routing"
)

type Router struct {
	routing.Routing

	Priority int // less = more important
}

type p2pRouterOut struct {
	fx.Out

	Router Router `group:"routers"`
}

type AddrInfoChan chan peer.AddrInfo

type p2pOnlineRoutingIn struct {
	fx.In

	Routers   []Router `group:"routers"`
	Validator record.Validator
}

// Routing will get all routers obtained from different methods
// (delegated routers, pub-sub, and so on) and add them all together
// using a TieredRouter.
func Routing(in p2pOnlineRoutingIn) irouting.TieredRouter {
	routers := in.Routers

	sort.SliceStable(routers, func(i, j int) bool {
		return routers[i].Priority < routers[j].Priority
	})

	irouters := make([]routing.Routing, len(routers))
	for i, v := range routers {
		irouters[i] = v.Routing
	}

	return irouting.Tiered{
		Tiered: routinghelpers.Tiered{
			Routers:   irouters,
			Validator: in.Validator,
		},
	}
}

// OfflineRouting provides a special Router to the routers list when we are creating a offline node.
func OfflineRouting(dstore ds.Datastore, validator record.Validator) p2pRouterOut {
	return p2pRouterOut{
		Router: Router{
			Routing:  offroute.NewOfflineRouter(dstore, validator),
			Priority: 10000,
		},
	}
}

type p2pPSRoutingIn struct {
	fx.In

	Validator record.Validator
	Host      host.Host
	PubSub    *pubsub.PubSub `optional:"true"`
}

func PubsubRouter(mctx helpers.MetricsCtx, lc fx.Lifecycle, in p2pPSRoutingIn) (p2pRouterOut, *namesys.PubsubValueStore, error) {
	psRouter, err := namesys.NewPubsubValueStore(
		helpers.LifecycleCtx(mctx, lc),
		in.Host,
		in.PubSub,
		in.Validator,
		namesys.WithRebroadcastInterval(time.Minute),
	)

	if err != nil {
		return p2pRouterOut{}, nil, err
	}

	return p2pRouterOut{
		Router: Router{
			Routing: &routinghelpers.Compose{
				ValueStore: &routinghelpers.LimitedValueStore{
					ValueStore: psRouter,
					Namespaces: []string{"ipns"},
				},
			},
			Priority: 100,
		},
	}, psRouter, nil
}

func AutoRelayFeeder(cfgPeering config.Peering) interface{} {
	return func(lc fx.Lifecycle, h host.Host, peerChan AddrInfoChan, router irouting.TieredRouter) {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})

		defer func() {
			if r := recover(); r != nil {
				fmt.Println("Recovering from unexpected error in AutoRelayFeeder:", r)
				debug.PrintStack()
			}
		}()
		go func() {
			defer close(done)

			// Feed peers more often right after the bootstrap, then backoff
			bo := backoff.NewExponentialBackOff()
			bo.InitialInterval = 15 * time.Second
			bo.Multiplier = 3
			bo.MaxInterval = 1 * time.Hour
			bo.MaxElapsedTime = 0 // never stop
			t := backoff.NewTicker(bo)
			defer t.Stop()
			for {
				select {
				case <-t.C:
				case <-ctx.Done():
					return
				}

				// Always feed trusted IDs (Peering.Peers in the config)
				for _, trustedPeer := range cfgPeering.Peers {
					if len(trustedPeer.Addrs) == 0 {
						continue
					}
					select {
					case peerChan <- trustedPeer:
					case <-ctx.Done():
						return
					}
				}

				closestPeers, err := router.GetClosestPeers(ctx, h.ID().String())
				if err != nil {
					// no-op: usually 'failed to find any peer in table' during startup
					continue
				}
				for _, p := range closestPeers {
					addrs := h.Peerstore().Addrs(p)
					if len(addrs) == 0 {
						continue
					}
					dhtPeer := peer.AddrInfo{ID: p, Addrs: addrs}
					select {
					case peerChan <- dhtPeer:
					case <-ctx.Done():
						return
					}
				}
			}
		}()

		lc.Append(fx.Hook{
			OnStop: func(_ context.Context) error {
				cancel()
				<-done
				return nil
			},
		})
	}
}
