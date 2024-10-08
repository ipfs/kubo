package libp2p

import (
	"context"
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
	LoopbackAddressesOnLanDHT     bool
}

type RoutingOption func(args RoutingOptionArgs) (routing.Routing, error)

func constructDefaultHTTPRouters(cfg *config.Config) ([]*routinghelpers.ParallelRouter, error) {
	var routers []*routinghelpers.ParallelRouter
	// Append HTTP routers for additional speed
	for _, endpoint := range config.DefaultHTTPRouters {
		httpRouter, err := irouting.ConstructHTTPRouter(endpoint, cfg.Identity.PeerID, httpAddrsFromConfig(cfg.Addresses), cfg.Identity.PrivKey)
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
			Router:                  r,
			IgnoreError:             true,             // https://github.com/ipfs/kubo/pull/9475#discussion_r1042507387
			Timeout:                 15 * time.Second, // 5x server value from https://github.com/ipfs/kubo/pull/9475#discussion_r1042428529
			DoNotWaitForSearchValue: true,
			ExecuteAfter:            0,
		})
	}
	return routers, nil
}

// ConstructDefaultRouting returns routers used when Routing.Type is unset or set to "auto"
func ConstructDefaultRouting(cfg *config.Config, routingOpt RoutingOption) RoutingOption {
	return func(args RoutingOptionArgs) (routing.Routing, error) {
		// Defined routers will be queried in parallel (optimizing for response speed)
		// Different trade-offs can be made by setting Routing.Type = "custom" with own Routing.Routers
		var routers []*routinghelpers.ParallelRouter

		dhtRouting, err := routingOpt(args)
		if err != nil {
			return nil, err
		}
		routers = append(routers, &routinghelpers.ParallelRouter{
			Router:                  dhtRouting,
			IgnoreError:             false,
			DoNotWaitForSearchValue: true,
			ExecuteAfter:            0,
		})

		httpRouters, err := constructDefaultHTTPRouters(cfg)
		if err != nil {
			return nil, err
		}

		routers = append(routers, httpRouters...)

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
		wanOptions := []dht.Option{
			dht.BootstrapPeers(args.BootstrapPeers...),
		}
		lanOptions := []dht.Option{}
		if args.LoopbackAddressesOnLanDHT {
			lanOptions = append(lanOptions, dht.AddressFilter(nil))
		}
		return dual.New(
			args.Ctx, args.Host,
			dual.DHTOption(dhtOpts...),
			dual.WanDHTOption(wanOptions...),
			dual.LanDHTOption(lanOptions...),
		)
	}
}

// ConstructDelegatedRouting is used when Routing.Type = "custom"
func ConstructDelegatedRouting(routers config.Routers, methods config.Methods, peerID string, addrs config.Addresses, privKey string) RoutingOption {
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
				Addrs:      httpAddrsFromConfig(addrs),
				PrivKeyB64: privKey,
			},
		)
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

// httpAddrsFromConfig creates a list of addresses from the provided configuration to be used by HTTP delegated routers.
func httpAddrsFromConfig(cfgAddrs config.Addresses) []string {
	// Swarm addrs are announced by default
	addrs := cfgAddrs.Swarm
	// if Announce addrs are specified - override Swarm
	if len(cfgAddrs.Announce) > 0 {
		addrs = cfgAddrs.Announce
	} else if len(cfgAddrs.NoAnnounce) > 0 {
		// if Announce adds are not specified - filter Swarm addrs with NoAnnounce list
		maddrs := map[string]struct{}{}
		for _, addr := range addrs {
			maddrs[addr] = struct{}{}
		}
		for _, addr := range cfgAddrs.NoAnnounce {
			delete(maddrs, addr)
		}
		addrs = make([]string, 0, len(maddrs))
		for k := range maddrs {
			addrs = append(addrs, k)
		}
	}
	// append AppendAnnounce addrs to the result list
	if len(cfgAddrs.AppendAnnounce) > 0 {
		addrs = append(addrs, cfgAddrs.AppendAnnounce...)
	}
	return addrs
}
