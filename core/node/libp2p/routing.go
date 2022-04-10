package libp2p

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ipfs/go-ipfs/core/node/helpers"

	"github.com/ipfs/go-ipfs/repo"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	ddht "github.com/libp2p/go-libp2p-kad-dht/dual"
	"github.com/libp2p/go-libp2p-kad-dht/fullrt"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	namesys "github.com/libp2p/go-libp2p-pubsub-router"
	record "github.com/libp2p/go-libp2p-record"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"

	"go.uber.org/fx"
)

type BaseIpfsRouting routing.Routing

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

	Router    Router `group:"routers"`
	DHT       *ddht.DHT
	DHTClient routing.Routing `name:"dhtc"`
	BaseRT    BaseIpfsRouting
}

func BaseRouting(experimentalDHTClient bool) interface{} {
	return func(mctx helpers.MetricsCtx, lc fx.Lifecycle, in processInitialRoutingIn) (out processInitialRoutingOut, err error) {
		var dr *ddht.DHT
		if dht, ok := in.Router.(*ddht.DHT); ok {
			dr = dht

			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					return dr.Close()
				},
			})
		}

		if dr != nil && experimentalDHTClient {
			cfg, err := in.Repo.Config()
			if err != nil {
				return out, err
			}
			bspeers, err := cfg.BootstrapPeers()
			if err != nil {
				return out, err
			}

			expClient, err := fullrt.NewFullRT(in.Host,
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
					return expClient.Close()
				},
			})

			return processInitialRoutingOut{
				Router: Router{
					Routing:  expClient,
					Priority: 1000,
				},
				DHT:       dr,
				DHTClient: expClient,
				BaseRT:    expClient,
			}, nil
		}

		return processInitialRoutingOut{
			Router: Router{
				Priority: 1000,
				Routing:  in.Router,
			},
			DHT:       dr,
			DHTClient: dr,
			BaseRT:    in.Router,
		}, nil
	}
}

type p2pOnlineRoutingIn struct {
	fx.In

	Routers   []Router `group:"routers"`
	Validator record.Validator
}

func Routing(in p2pOnlineRoutingIn) routing.Routing {
	routers := in.Routers

	sort.SliceStable(routers, func(i, j int) bool {
		return routers[i].Priority < routers[j].Priority
	})

	irouters := make([]routing.Routing, len(routers))
	for i, v := range routers {
		irouters[i] = v.Routing
	}

	return routinghelpers.Tiered{
		Routers:   irouters,
		Validator: in.Validator,
	}
}

type p2pPSRoutingIn struct {
	fx.In

	BaseIpfsRouting BaseIpfsRouting
	Validator       record.Validator
	Host            host.Host
	PubSub          *pubsub.PubSub `optional:"true"`
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

func AutoRelayFeeder(lc fx.Lifecycle, h host.Host, peerChan chan peer.AddrInfo, dht *ddht.DHT) {
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)

		t := time.NewTicker(time.Minute / 3) // TODO: this is way too frequent. Should probably use some kind of backoff
		defer t.Stop()
		for {
			select {
			case <-t.C:
			case <-ctx.Done():
				return
			}
			closestPeers, err := dht.WAN.GetClosestPeers(ctx, h.ID().String())
			if err != nil {
				fmt.Println(err)
				continue
			}
			for _, p := range closestPeers {
				addrs := h.Peerstore().Addrs(p)
				if len(addrs) == 0 {
					continue
				}
				select {
				case peerChan <- peer.AddrInfo{ID: p, Addrs: addrs}:
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
