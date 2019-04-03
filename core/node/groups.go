package node

import (
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	"go.uber.org/fx"

	offroute "github.com/ipfs/go-ipfs-routing/offline"
	"github.com/ipfs/go-ipfs/p2p"
	"github.com/ipfs/go-ipfs/provider"
)

var LibP2P = fx.Options(
	fx.Provide(P2PAddrFilters),
	fx.Provide(P2PBandwidthCounter),
	fx.Provide(P2PPNet),
	fx.Provide(P2PAddrsFactory),
	fx.Provide(P2PConnectionManager),
	fx.Provide(P2PSmuxTransport),
	fx.Provide(P2PNatPortMap),
	fx.Provide(P2PRelay),
	fx.Provide(P2PAutoRealy),
	fx.Provide(P2PDefaultTransports),
	fx.Provide(P2PQUIC),

	fx.Provide(P2PHostOption),
	fx.Provide(P2PHost),
	fx.Provide(P2POnlineRouting),

	fx.Provide(Pubsub),
	fx.Provide(NewDiscoveryHandler),

	fx.Invoke(AutoNATService),
	fx.Invoke(P2PPNetChecker),
	fx.Invoke(StartListening),
	fx.Invoke(SetupDiscovery),
)

var Storage = fx.Options(
	fx.Provide(RepoConfig),
	fx.Provide(DatastoreCtor),
	fx.Provide(BaseBlockstoreCtor),
	fx.Provide(GcBlockstoreCtor),
)

var Identity = fx.Options(
	fx.Provide(PeerID),
	fx.Provide(PrivateKey),
	fx.Provide(Peerstore),
)

var IPNS = fx.Options(
	fx.Provide(RecordValidator),
)

var Providers = fx.Options(
	fx.Provide(ProviderQueue),
	fx.Provide(ProviderCtor),
	fx.Provide(ReproviderCtor),

	fx.Invoke(Reprovider),
	fx.Invoke(provider.Provider.Run),
)

var Online = fx.Options(
	fx.Provide(OnlineExchangeCtor),
	fx.Provide(OnlineNamesysCtor),

	fx.Invoke(IpnsRepublisher),

	fx.Provide(p2p.NewP2P),

	LibP2P,
	Providers,
)

var Offline = fx.Options(
	fx.Provide(offline.Exchange),
	fx.Provide(OfflineNamesysCtor),
	fx.Provide(offroute.NewOfflineRouter),
	fx.Provide(provider.NewOfflineProvider),
)

func Networked(online bool) fx.Option {
	if online {
		return Online
	}
	return Offline
}
