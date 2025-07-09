package libp2p

import (
	"context"
	"fmt"
	"runtime/debug"
	"sort"
	"time"

	"github.com/cenkalti/backoff/v4"
	offroute "github.com/ipfs/boxo/routing/offline"
	ds "github.com/ipfs/go-datastore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	ddht "github.com/libp2p/go-libp2p-kad-dht/dual"
	"github.com/libp2p/go-libp2p-kad-dht/fullrt"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	namesys "github.com/libp2p/go-libp2p-pubsub-router"
	record "github.com/libp2p/go-libp2p-record"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"go.uber.org/fx"

	config "github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/node/helpers"
	"github.com/ipfs/kubo/repo"
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

type processInitialRoutingIn struct {
	fx.In

	Router routing.Routing `name:"initialrouting"`

	// For setting up experimental DHT client
	Host      host.Host
	Repo      repo.Repo
	Validator record.Validator
}

type processInitialRoutingOut struct {
	fx.Out

	Router        Router                 `group:"routers"`
	ContentRouter routing.ContentRouting `group:"content-routers"`

	DHT       *ddht.DHT
	DHTClient routing.Routing `name:"dhtc"`
}

type AddrInfoChan chan peer.AddrInfo

func BaseRouting(cfg *config.Config) interface{} {
	return func(lc fx.Lifecycle, in processInitialRoutingIn) (out processInitialRoutingOut, err error) {
		var dualDHT *ddht.DHT
		if dht, ok := in.Router.(*ddht.DHT); ok {
			dualDHT = dht

			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					return dualDHT.Close()
				},
			})
		}

		if cr, ok := in.Router.(routinghelpers.ComposableRouter); ok {
			for _, r := range cr.Routers() {
				if dht, ok := r.(*ddht.DHT); ok {
					dualDHT = dht
					lc.Append(fx.Hook{
						OnStop: func(ctx context.Context) error {
							return dualDHT.Close()
						},
					})
					break
				}
			}
		}

		if dualDHT != nil && cfg.Routing.AcceleratedDHTClient.WithDefault(config.DefaultAcceleratedDHTClient) {
			cfg, err := in.Repo.Config()
			if err != nil {
				return out, err
			}
			bspeers, err := cfg.BootstrapPeers()
			if err != nil {
				return out, err
			}

			fullRTClient, err := fullrt.NewFullRT(in.Host,
				dht.DefaultPrefix,
				fullrt.DHTOption(
					dht.Validator(in.Validator),
					dht.Datastore(in.Repo.Datastore()),
					dht.BootstrapPeers(bspeers...),
					dht.BucketSize(20),
				),
			)
			if err != nil {
				return out, err
			}

			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					return fullRTClient.Close()
				},
			})

			// we want to also use the default HTTP routers, so wrap the FullRT client
			// in a parallel router that calls them in parallel
			httpRouters, err := constructDefaultHTTPRouters(cfg)
			if err != nil {
				return out, err
			}
			routers := []*routinghelpers.ParallelRouter{
				{Router: fullRTClient, DoNotWaitForSearchValue: true},
			}
			routers = append(routers, httpRouters...)
			router := routinghelpers.NewComposableParallel(routers)

			return processInitialRoutingOut{
				Router: Router{
					Priority: 1000,
					Routing:  router,
				},
				DHT:           dualDHT,
				DHTClient:     fullRTClient,
				ContentRouter: fullRTClient,
			}, nil
		}

		return processInitialRoutingOut{
			Router: Router{
				Priority: 1000,
				Routing:  in.Router,
			},
			DHT:           dualDHT,
			DHTClient:     dualDHT,
			ContentRouter: in.Router,
		}, nil
	}
}

type p2pOnlineContentRoutingIn struct {
	fx.In

	ContentRouter []routing.ContentRouting `group:"content-routers"`
}

// ContentRouting will get all routers that can do contentRouting and add them
// all together using a TieredRouter. It will be used for topic discovery.
func ContentRouting(in p2pOnlineContentRoutingIn) routing.ContentRouting {
	var routers []routing.Routing
	for _, cr := range in.ContentRouter {
		routers = append(routers,
			&routinghelpers.Compose{
				ContentRouting: cr,
			},
		)
	}

	return routinghelpers.Tiered{
		Routers: routers,
	}
}

type p2pOnlineRoutingIn struct {
	fx.In

	Routers   []Router `group:"routers"`
	Validator record.Validator
}

// Routing will get all routers obtained from different methods (delegated
// routers, pub-sub, and so on) and add them all together using a TieredRouter.
func Routing(in p2pOnlineRoutingIn) irouting.ProvideManyRouter {
	routers := in.Routers

	sort.SliceStable(routers, func(i, j int) bool {
		return routers[i].Priority < routers[j].Priority
	})

	var cRouters []*routinghelpers.ParallelRouter
	for _, v := range routers {
		cRouters = append(cRouters, &routinghelpers.ParallelRouter{
			IgnoreError:             true,
			DoNotWaitForSearchValue: true,
			Router:                  v.Routing,
		})
	}

	return routinghelpers.NewComposableParallel(cRouters)
}

// OfflineRouting provides a special Router to the routers list when we are
// creating an offline node.
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

func autoRelayFeeder(cfgPeering config.Peering, peerChan chan<- peer.AddrInfo) fx.Option {
	return fx.Invoke(func(lc fx.Lifecycle, h host.Host, dht *ddht.DHT) {
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

				// Additionally, feed closest peers discovered via DHT
				if dht == nil {
					/* noop due to missing dht.WAN. happens in some unit tests,
					   not worth fixing as we will refactor this after go-libp2p 0.20 */
					continue
				}
				closestPeers, err := dht.WAN.GetClosestPeers(ctx, h.ID().String())
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
	})
}
