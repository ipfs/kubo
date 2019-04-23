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
	fx.Provide(libp2p.AddrFilters),
	fx.Provide(libp2p.BandwidthCounter),
	fx.Provide(libp2p.PNet),
	fx.Provide(libp2p.AddrsFactory),
	fx.Provide(libp2p.ConnectionManager),
	fx.Provide(libp2p.NatPortMap),
	fx.Provide(libp2p.Relay),
	fx.Provide(libp2p.AutoRealy),
	fx.Provide(libp2p.DefaultTransports),
	fx.Provide(libp2p.QUIC),

	fx.Provide(libp2p.Host),

	fx.Provide(libp2p.DiscoveryHandler),

	fx.Invoke(libp2p.AutoNATService),
	fx.Invoke(libp2p.PNetChecker),
	fx.Invoke(libp2p.StartListening),
	fx.Invoke(libp2p.SetupDiscovery),
)

func LibP2P(cfg *BuildCfg) fx.Option {
	opts := fx.Options(
		BaseLibP2P,

		fx.Provide(libp2p.Security(!cfg.DisableEncryptedConnections)),
		maybeProvide(libp2p.Pubsub, cfg.getOpt("pubsub") || cfg.getOpt("ipnsps")),

		fx.Provide(libp2p.SmuxTransport(cfg.getOpt("mplex"))),
		fx.Provide(libp2p.Routing),
		fx.Provide(libp2p.BaseRouting),
		maybeProvide(libp2p.PubsubRouter, cfg.getOpt("ipnsps")),
	)

	return opts
}

// Storage groups units which setup datastore based persistence and blockstore layers
func Storage(cfg *BuildCfg) fx.Option {
	return fx.Options(
		fx.Provide(RepoConfig),
		fx.Provide(Datastore),
		fx.Provide(BaseBlockstoreCtor(cfg.Permanent, cfg.NilRepo)),
		fx.Provide(GcBlockstoreCtor),
	)
}

// Identity groups units providing cryptographic identity
var Identity = fx.Options(
	fx.Provide(PeerID),
	fx.Provide(PrivateKey),
	fx.Provide(libp2p.Peerstore),
)

// IPNS groups namesys related units
var IPNS = fx.Options(
	fx.Provide(RecordValidator),
)

// Providers groups units managing provider routing records
var Providers = fx.Options(
	fx.Provide(ProviderQueue),
	fx.Provide(ProviderCtor),
	fx.Provide(ReproviderCtor),

	fx.Invoke(Reprovider),
)

// Online groups online-only units
func Online(cfg *BuildCfg) fx.Option {
	return fx.Options(
		fx.Provide(OnlineExchange),
		fx.Provide(OnlineNamesys),

		fx.Invoke(IpnsRepublisher),

		fx.Provide(p2p.New),

		LibP2P(cfg),
		Providers,
	)
}

// Offline groups offline alternatives to Online units
var Offline = fx.Options(
	fx.Provide(offline.Exchange),
	fx.Provide(OfflineNamesys),
	fx.Provide(offroute.NewOfflineRouter),
	fx.Provide(provider.NewOfflineProvider),
)

// Core groups basic IPFS services
var Core = fx.Options(
	fx.Provide(BlockService),
	fx.Provide(Dag),
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

// IPFS builds a group of fx Options based on the passed BuildCfg
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
