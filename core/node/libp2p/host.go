package libp2p

import (
	"context"

	"github.com/libp2p/go-libp2p"
	record "github.com/libp2p/go-libp2p-record"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/routing"
	routedhost "github.com/libp2p/go-libp2p/p2p/host/routed"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/node/helpers"
	"github.com/ipfs/kubo/repo"

	"go.uber.org/fx"
)

type P2PHostIn struct {
	fx.In

	Repo          repo.Repo
	Validator     record.Validator
	HostOption    HostOption
	RoutingOption RoutingOption
	ID            peer.ID
	Peerstore     peerstore.Peerstore

	Opts [][]libp2p.Option `group:"libp2p"`
}

type P2PHostOut struct {
	fx.Out

	Host    host.Host
	Routing routing.Routing `name:"initialrouting"`
}

func Host(mctx helpers.MetricsCtx, lc fx.Lifecycle, params P2PHostIn) (out P2PHostOut, err error) {
	opts := []libp2p.Option{libp2p.NoListenAddrs}
	for _, o := range params.Opts {
		opts = append(opts, o...)
	}

	ctx := helpers.LifecycleCtx(mctx, lc)
	cfg, err := params.Repo.Config()
	if err != nil {
		return out, err
	}
	// Use auto-config resolution for actual connectivity
	bootstrappers, err := cfg.BootstrapPeersWithAutoConf()
	if err != nil {
		return out, err
	}

	// Optimistic provide is enabled either via dedicated expierimental flag, or when DHT Provide Sweep is enabled.
	// When DHT Provide Sweep is enabled, all provide operations go through the
	// `SweepingProvider`, hence the provides don't use the optimistic provide
	// logic. Provides use `SweepingProvider.StartProviding()` and not
	// `IpfsDHT.Provide()`, which is where the optimistic provide logic is
	// implemented. However, `IpfsDHT.Provide()` is used to quickly provide roots
	// when user manually adds content with the `--fast-provide` flag enabled. In
	// this case we want to use optimistic provide logic to quickly announce the
	// content to the network. This should be the only use case of
	// `IpfsDHT.Provide()` when DHT Provide Sweep is enabled.
	optimisticProvide := cfg.Experimental.OptimisticProvide || cfg.Provide.DHT.SweepEnabled.WithDefault(config.DefaultProvideDHTSweepEnabled)

	routingOptArgs := RoutingOptionArgs{
		Ctx:                           ctx,
		Datastore:                     params.Repo.Datastore(),
		Validator:                     params.Validator,
		BootstrapPeers:                bootstrappers,
		OptimisticProvide:             optimisticProvide,
		OptimisticProvideJobsPoolSize: cfg.Experimental.OptimisticProvideJobsPoolSize,
		LoopbackAddressesOnLanDHT:     cfg.Routing.LoopbackAddressesOnLanDHT.WithDefault(config.DefaultLoopbackAddressesOnLanDHT),
	}
	opts = append(opts, libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
		args := routingOptArgs
		args.Host = h
		r, err := params.RoutingOption(args)
		out.Routing = r
		return r, err
	}))

	out.Host, err = params.HostOption(params.ID, params.Peerstore, opts...)
	if err != nil {
		return P2PHostOut{}, err
	}

	routingOptArgs.Host = out.Host

	// this code is necessary just for tests: mock network constructions
	// ignore the libp2p constructor options that actually construct the routing!
	if out.Routing == nil {
		r, err := params.RoutingOption(routingOptArgs)
		if err != nil {
			return P2PHostOut{}, err
		}
		out.Routing = r
		out.Host = routedhost.Wrap(out.Host, out.Routing)
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return out.Host.Close()
		},
	})

	return out, err
}
