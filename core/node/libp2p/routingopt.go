package libp2p

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/kubo/config"
	irouting "github.com/ipfs/kubo/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	dual "github.com/libp2p/go-libp2p-kad-dht/dual"
	record "github.com/libp2p/go-libp2p-record"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	host "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	routing "github.com/libp2p/go-libp2p/core/routing"
)

type RoutingOptionArgs struct {
	Ctx                           context.Context
	Host                          host.Host
	Datastore                     datastore.Batching
	Validator                     record.Validator
	BootstrapPeers                []peer.AddrInfo
	OptimisticProvide             bool
	OptimisticProvideJobsPoolSize int
}

type RoutingOption func(args RoutingOptionArgs) (routing.Routing, error)

// Default HTTP routers used in parallel to DHT when Routing.Type = "auto"
var defaultHTTPRouters = []string{
	"https://cid.contact", // https://github.com/ipfs/kubo/issues/9422#issuecomment-1338142084
	// TODO: add an independent router from Cloudflare
}

func init() {
	// Override HTTP routers if custom ones were passed via env
	if routers := os.Getenv("IPFS_HTTP_ROUTERS"); routers != "" {
		defaultHTTPRouters = strings.Split(routers, " ")
	}
}

// ConstructDefaultRouting returns routers used when Routing.Type is unset or set to "auto"
func ConstructDefaultRouting(peerID string, addrs []string, privKey string, routingOpt RoutingOption) RoutingOption {
	return func(args RoutingOptionArgs) (routing.Routing, error) {
		// Defined routers will be queried in parallel (optimizing for response speed)
		// Different trade-offs can be made by setting Routing.Type = "custom" with own Routing.Routers
		var routers []*routinghelpers.ParallelRouter

		dhtRouting, err := routingOpt(args)
		if err != nil {
			return nil, err
		}
		routers = append(routers, &routinghelpers.ParallelRouter{
			Router:       dhtRouting,
			IgnoreError:  false,
			ExecuteAfter: 0,
		})

		// Append HTTP routers for additional speed
		for _, endpoint := range defaultHTTPRouters {
			httpRouter, err := irouting.ConstructHTTPRouter(endpoint, peerID, addrs, privKey)
			if err != nil {
				return nil, err
			}

			r := &irouting.Composer{
				GetValueRouter:      routinghelpers.Null{},
				PutValueRouter:      routinghelpers.Null{},
				ProvideRouter:       routinghelpers.Null{}, // modify this when indexers supports provide
				FindPeersRouter:     routinghelpers.Null{},
				FindProvidersRouter: httpRouter,
			}

			routers = append(routers, &routinghelpers.ParallelRouter{
				Router:       r,
				IgnoreError:  true,             // https://github.com/ipfs/kubo/pull/9475#discussion_r1042507387
				Timeout:      15 * time.Second, // 5x server value from https://github.com/ipfs/kubo/pull/9475#discussion_r1042428529
				ExecuteAfter: 0,
			})
		}

		routing := routinghelpers.NewComposableParallel(routers)
		return routing, nil
	}
}

// constructDHTRouting is used when Routing.Type = "dht"
func constructDHTRouting(mode dht.ModeOpt) RoutingOption {
	return func(args RoutingOptionArgs) (routing.Routing, error) {
		dhtOpts := []dht.Option{
			dht.Concurrency(10),
			dht.Mode(mode),
			dht.Datastore(args.Datastore),
			dht.Validator(args.Validator),
		}
		if args.OptimisticProvide {
			dhtOpts = append(dhtOpts, dht.EnableOptimisticProvide())
		}
		if args.OptimisticProvideJobsPoolSize != 0 {
			dhtOpts = append(dhtOpts, dht.OptimisticProvideJobsPoolSize(args.OptimisticProvideJobsPoolSize))
		}
		return dual.New(
			args.Ctx, args.Host,
			dual.DHTOption(dhtOpts...),
			dual.WanDHTOption(dht.BootstrapPeers(args.BootstrapPeers...)),
		)
	}
}

// ConstructDelegatedRouting is used when Routing.Type = "custom"
func ConstructDelegatedRouting(routers config.Routers, methods config.Methods, peerID string, addrs []string, privKey string) RoutingOption {
	return func(args RoutingOptionArgs) (routing.Routing, error) {
		return irouting.Parse(routers, methods,
			&irouting.ExtraDHTParams{
				BootstrapPeers: args.BootstrapPeers,
				Host:           args.Host,
				Validator:      args.Validator,
				Datastore:      args.Datastore,
				Context:        args.Ctx,
			},
			&irouting.ExtraHTTPParams{
				PeerID:     peerID,
				Addrs:      addrs,
				PrivKeyB64: privKey,
			})
	}
}

func constructNilRouting(_ RoutingOptionArgs) (routing.Routing, error) {
	return routinghelpers.Null{}, nil
}

var (
	DHTOption       RoutingOption = constructDHTRouting(dht.ModeAuto)
	DHTClientOption               = constructDHTRouting(dht.ModeClient)
	DHTServerOption               = constructDHTRouting(dht.ModeServer)
	NilRouterOption               = constructNilRouting
)
