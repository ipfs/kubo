package node

import (
	"context"

	"github.com/ipfs/go-ipfs/core/node/libp2p"
	"github.com/ipfs/go-ipfs/p2p"
	"github.com/ipfs/go-ipfs/provider"

	offline "github.com/ipfs/go-ipfs-exchange-offline"
	offroute "github.com/ipfs/go-ipfs-routing/offline"
	"github.com/ipfs/go-path/resolver"
	"go.uber.org/fx"
)

var BaseLibP2P = fx.Options(
	fx.Provide(libp2p.P2PAddrFilters),
	fx.Provide(libp2p.P2PBandwidthCounter),
	fx.Provide(libp2p.P2PPNet),
	fx.Provide(libp2p.P2PAddrsFactory),
	fx.Provide(libp2p.P2PConnectionManager),
	fx.Provide(libp2p.P2PNatPortMap),
	fx.Provide(libp2p.P2PRelay),
	fx.Provide(libp2p.P2PAutoRealy),
	fx.Provide(libp2p.P2PDefaultTransports),
	fx.Provide(libp2p.P2PQUIC),

	fx.Provide(libp2p.P2PHost),

	fx.Provide(libp2p.NewDiscoveryHandler),

	fx.Invoke(libp2p.AutoNATService),
	fx.Invoke(libp2p.P2PPNetChecker),
	fx.Invoke(libp2p.StartListening),
	fx.Invoke(libp2p.SetupDiscovery),
)

func LibP2P(cfg *BuildCfg) fx.Option {
	opts := fx.Options(
		BaseLibP2P,

		fx.Provide(libp2p.P2PSecurity(!cfg.DisableEncryptedConnections)),
		maybeProvide(libp2p.Pubsub, cfg.getOpt("pubsub") || cfg.getOpt("ipnsps")),

		fx.Provide(libp2p.P2PSmuxTransport(cfg.getOpt("mplex"))),
		fx.Provide(libp2p.P2PRouting),
		fx.Provide(libp2p.P2PBaseRouting),
		maybeProvide(libp2p.P2PPubsubRouter, cfg.getOpt("ipnsps")),
	)

	return opts
}

func Storage(cfg *BuildCfg) fx.Option {
	return fx.Options(
		fx.Provide(RepoConfig),
		fx.Provide(DatastoreCtor),
		fx.Provide(BaseBlockstoreCtor(cfg.Permanent, cfg.NilRepo)),
		fx.Provide(GcBlockstoreCtor),
	)
}

var Identity = fx.Options(
	fx.Provide(PeerID),
	fx.Provide(PrivateKey),
	fx.Provide(libp2p.Peerstore),
)

var IPNS = fx.Options(
	fx.Provide(RecordValidator),
)

var Providers = fx.Options(
	fx.Provide(ProviderQueue),
	fx.Provide(ProviderCtor),
	fx.Provide(ReproviderCtor),

	fx.Invoke(Reprovider),
)

func Online(cfg *BuildCfg) fx.Option {
	return fx.Options(
		fx.Provide(OnlineExchangeCtor),
		fx.Provide(OnlineNamesysCtor),

		fx.Invoke(IpnsRepublisher),

		fx.Provide(p2p.NewP2P),

		LibP2P(cfg),
		Providers,
	)
}

var Offline = fx.Options(
	fx.Provide(offline.Exchange),
	fx.Provide(OfflineNamesysCtor),
	fx.Provide(offroute.NewOfflineRouter),
	fx.Provide(provider.NewOfflineProvider),
)

var Core = fx.Options(
	fx.Provide(BlockServiceCtor),
	fx.Provide(DagCtor),
	fx.Provide(resolver.NewBasicResolver),
	fx.Provide(Pinning),
	fx.Provide(Files),
)

func Networked(cfg *BuildCfg) fx.Option {
	if cfg.Online {
		return Online(cfg)
	}
	return Offline
}

func IPFS(ctx context.Context, cfg *BuildCfg) fx.Option {
	if cfg == nil {
		cfg = new(BuildCfg)
	}

	return fx.Options(
		cfg.options(ctx),

		fx.Provide(baseProcess),
		fx.Invoke(setupSharding),

		Storage(cfg),
		Identity,
		IPNS,
		Networked(cfg),

		Core,
	)
}
