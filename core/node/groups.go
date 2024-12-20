package node

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	blockstore "github.com/ipfs/boxo/blockstore"
	offline "github.com/ipfs/boxo/exchange/offline"
	uio "github.com/ipfs/boxo/ipld/unixfs/io"
	util "github.com/ipfs/boxo/util"
	"github.com/ipfs/go-log"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/p2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p-pubsub/timecache"
	"github.com/libp2p/go-libp2p/core/peer"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"go.uber.org/fx"
)

var logger = log.Logger("core:constructor")

var BaseLibP2P = fx.Options(
	fx.Provide(libp2p.PNet),
	fx.Provide(libp2p.ConnectionManager),
	fx.Provide(libp2p.Host),
	fx.Provide(libp2p.MultiaddrResolver),

	fx.Provide(libp2p.DiscoveryHandler),

	fx.Invoke(libp2p.PNetChecker),
)

func LibP2P(bcfg *BuildCfg, cfg *config.Config, userResourceOverrides rcmgr.PartialLimitConfig) fx.Option {
	var connmgr fx.Option

	// set connmgr based on Swarm.ConnMgr.Type
	connMgrType := cfg.Swarm.ConnMgr.Type.WithDefault(config.DefaultConnMgrType)
	switch connMgrType {
	case "none":
		connmgr = fx.Options() // noop
	case "", "basic":
		grace := cfg.Swarm.ConnMgr.GracePeriod.WithDefault(config.DefaultConnMgrGracePeriod)
		low := int(cfg.Swarm.ConnMgr.LowWater.WithDefault(config.DefaultConnMgrLowWater))
		high := int(cfg.Swarm.ConnMgr.HighWater.WithDefault(config.DefaultConnMgrHighWater))
		connmgr = fx.Provide(libp2p.ConnectionManager(low, high, grace))
	default:
		return fx.Error(fmt.Errorf("unrecognized Swarm.ConnMgr.Type: %q", connMgrType))
	}

	// parse PubSub config

	ps, disc := fx.Options(), fx.Options()
	if bcfg.getOpt("pubsub") || bcfg.getOpt("ipnsps") {
		disc = fx.Provide(libp2p.TopicDiscovery())

		var pubsubOptions []pubsub.Option
		pubsubOptions = append(
			pubsubOptions,
			pubsub.WithMessageSigning(!cfg.Pubsub.DisableSigning),
			pubsub.WithSeenMessagesTTL(cfg.Pubsub.SeenMessagesTTL.WithDefault(pubsub.TimeCacheDuration)),
		)

		var seenMessagesStrategy timecache.Strategy
		configSeenMessagesStrategy := cfg.Pubsub.SeenMessagesStrategy.WithDefault(config.DefaultSeenMessagesStrategy)
		switch configSeenMessagesStrategy {
		case config.LastSeenMessagesStrategy:
			seenMessagesStrategy = timecache.Strategy_LastSeen
		case config.FirstSeenMessagesStrategy:
			seenMessagesStrategy = timecache.Strategy_FirstSeen
		default:
			return fx.Error(fmt.Errorf("unsupported Pubsub.SeenMessagesStrategy %q", configSeenMessagesStrategy))
		}
		pubsubOptions = append(pubsubOptions, pubsub.WithSeenMessagesStrategy(seenMessagesStrategy))

		switch cfg.Pubsub.Router {
		case "":
			fallthrough
		case "gossipsub":
			ps = fx.Provide(libp2p.GossipSub(pubsubOptions...))
		case "floodsub":
			ps = fx.Provide(libp2p.FloodSub(pubsubOptions...))
		default:
			return fx.Error(fmt.Errorf("unknown pubsub router %s", cfg.Pubsub.Router))
		}
	}

	autonat := fx.Options()

	switch cfg.AutoNAT.ServiceMode {
	default:
		panic("BUG: unhandled autonat service mode")
	case config.AutoNATServiceDisabled:
	case config.AutoNATServiceUnset:
		// TODO
		//
		// We're enabling the AutoNAT service by default on _all_ nodes
		// for the moment.
		//
		// We should consider disabling it by default if the dht is set
		// to dhtclient.
		fallthrough
	case config.AutoNATServiceEnabled:
		autonat = fx.Provide(libp2p.AutoNATService(cfg.AutoNAT.Throttle, false))
	case config.AutoNATServiceEnabledV1Only:
		autonat = fx.Provide(libp2p.AutoNATService(cfg.AutoNAT.Throttle, true))
	}

	enableRelayTransport := cfg.Swarm.Transports.Network.Relay.WithDefault(true) // nolint
	enableRelayService := cfg.Swarm.RelayService.Enabled.WithDefault(enableRelayTransport)
	enableRelayClient := cfg.Swarm.RelayClient.Enabled.WithDefault(enableRelayTransport)
	enableAutoTLS := cfg.AutoTLS.Enabled.WithDefault(config.DefaultAutoTLSEnabled)
	enableAutoWSS := cfg.AutoTLS.AutoWSS.WithDefault(config.DefaultAutoWSS)
	atlsLog := log.Logger("autotls")

	// Log error when relay subsystem could not be initialized due to missing dependency
	if !enableRelayTransport {
		if enableRelayService {
			logger.Fatal("Failed to enable `Swarm.RelayService`, it requires `Swarm.Transports.Network.Relay` to be true.")
		}
		if enableRelayClient {
			logger.Fatal("Failed to enable `Swarm.RelayClient`, it requires `Swarm.Transports.Network.Relay` to be true.")
		}
	}

	if enableAutoTLS {
		if !cfg.Swarm.Transports.Network.TCP.WithDefault(true) {
			logger.Fatal("Invalid configuration: AutoTLS.Enabled=true requires Swarm.Transports.Network.TCP to be true as well.")
		}
		if !cfg.Swarm.Transports.Network.Websocket.WithDefault(true) {
			logger.Fatal("Invalid configuration: AutoTLS.Enabled=true requires Swarm.Transports.Network.Websocket to be true as well.")
		}

		// AutoTLS for Secure WebSockets: ensure WSS listeners are in place (manual or automatic)
		wssWildcard := fmt.Sprintf("/tls/sni/*.%s/ws", cfg.AutoTLS.DomainSuffix.WithDefault(config.DefaultDomainSuffix))
		wssWildcardPresent := false
		customWsPresent := false
		customWsRegex := regexp.MustCompile(`/wss?$`)
		tcpRegex := regexp.MustCompile(`/tcp/\d+$`)

		// inspect listeners defined in config at Addresses.Swarm
		var tcpListeners []string
		for _, listener := range cfg.Addresses.Swarm {
			// detect if user manually added /tls/sni/.../ws listener matching AutoTLS.DomainSuffix
			if strings.Contains(listener, wssWildcard) {
				atlsLog.Infof("found compatible wildcard listener in Addresses.Swarm. AutoTLS will be used on %s", listener)
				wssWildcardPresent = true
				break
			}
			// detect if user manually added own /ws or /wss listener that is
			// not related to AutoTLS feature
			if customWsRegex.MatchString(listener) {
				atlsLog.Infof("found custom /ws listener set by user in Addresses.Swarm. AutoTLS will not be used on %s.", listener)
				customWsPresent = true
				break
			}
			// else, remember /tcp listeners that can be reused for /tls/sni/../ws
			if tcpRegex.MatchString(listener) {
				tcpListeners = append(tcpListeners, listener)
			}
		}

		// Append AutoTLS's wildcard listener
		// if no manual /ws listener was set by the user
		if enableAutoWSS && !wssWildcardPresent && !customWsPresent {
			if len(tcpListeners) == 0 {
				logger.Fatal("Invalid configuration: AutoTLS.AutoWSS=true requires at least one /tcp listener present in Addresses.Swarm, see https://github.com/ipfs/kubo/blob/master/docs/config.md#autotls")
			}
			for _, tcpListener := range tcpListeners {
				wssListener := tcpListener + wssWildcard
				cfg.Addresses.Swarm = append(cfg.Addresses.Swarm, wssListener)
				atlsLog.Infof("appended AutoWSS listener: %s", wssListener)
			}
		}

		if !wssWildcardPresent && !enableAutoWSS {
			logger.Fatal(fmt.Sprintf("Invalid configuration: AutoTLS.Enabled=true requires a /tcp listener ending with %q to be present in Addresses.Swarm or AutoTLS.AutoWSS=true, see https://github.com/ipfs/kubo/blob/master/docs/config.md#autotls", wssWildcard))
		}
	}

	// Gather all the options
	opts := fx.Options(
		BaseLibP2P,

		// identify's AgentVersion (incl. optional agent-version-suffix)
		fx.Provide(libp2p.UserAgent()),

		// Services (resource management)
		fx.Provide(libp2p.ResourceManager(bcfg.Repo.Path(), cfg.Swarm, userResourceOverrides)),
		maybeProvide(libp2p.P2PForgeCertMgr(bcfg.Repo.Path(), cfg.AutoTLS, atlsLog), enableAutoTLS),
		maybeInvoke(libp2p.StartP2PAutoTLS, enableAutoTLS),
		fx.Provide(libp2p.AddrFilters(cfg.Swarm.AddrFilters)),
		fx.Provide(libp2p.AddrsFactory(cfg.Addresses.Announce, cfg.Addresses.AppendAnnounce, cfg.Addresses.NoAnnounce)),
		fx.Provide(libp2p.SmuxTransport(cfg.Swarm.Transports)),
		fx.Provide(libp2p.RelayTransport(enableRelayTransport)),
		fx.Provide(libp2p.RelayService(enableRelayService, cfg.Swarm.RelayService)),
		fx.Provide(libp2p.Transports(cfg.Swarm.Transports)),
		fx.Provide(libp2p.ListenOn(cfg.Addresses.Swarm)),
		fx.Invoke(libp2p.SetupDiscovery(cfg.Discovery.MDNS.Enabled)),
		fx.Provide(libp2p.ForceReachability(cfg.Internal.Libp2pForceReachability)),
		fx.Provide(libp2p.HolePunching(cfg.Swarm.EnableHolePunching, enableRelayClient)),

		fx.Provide(libp2p.Security(!bcfg.DisableEncryptedConnections, cfg.Swarm.Transports)),

		fx.Provide(libp2p.Routing),
		fx.Provide(libp2p.ContentRouting),

		fx.Provide(libp2p.BaseRouting(cfg)),
		maybeProvide(libp2p.PubsubRouter, bcfg.getOpt("ipnsps")),

		maybeProvide(libp2p.BandwidthCounter, !cfg.Swarm.DisableBandwidthMetrics),
		maybeProvide(libp2p.NatPortMap, !cfg.Swarm.DisableNatPortMap),
		libp2p.MaybeAutoRelay(cfg.Swarm.RelayClient.StaticRelays, cfg.Peering, enableRelayClient),
		autonat,
		connmgr,
		ps,
		disc,
	)

	return opts
}

// Storage groups units which setup datastore based persistence and blockstore layers
func Storage(bcfg *BuildCfg, cfg *config.Config) fx.Option {
	cacheOpts := blockstore.DefaultCacheOpts()
	cacheOpts.HasBloomFilterSize = cfg.Datastore.BloomFilterSize
	cacheOpts.HasTwoQueueCacheSize = int(cfg.Datastore.BlockKeyCacheSize.WithDefault(config.DefaultBlockKeyCacheSize))
	if !bcfg.Permanent {
		cacheOpts.HasBloomFilterSize = 0
	}

	finalBstore := fx.Provide(GcBlockstoreCtor)
	if cfg.Experimental.FilestoreEnabled || cfg.Experimental.UrlstoreEnabled {
		finalBstore = fx.Provide(FilestoreBlockstoreCtor)
	}

	return fx.Options(
		fx.Provide(RepoConfig),
		fx.Provide(Datastore),
		fx.Provide(BaseBlockstoreCtor(cacheOpts, cfg.Datastore.HashOnRead, cfg.Datastore.WriteThrough.WithDefault(config.DefaultWriteThrough))),
		finalBstore,
	)
}

// Identity groups units providing cryptographic identity
func Identity(cfg *config.Config) fx.Option {
	// PeerID

	cid := cfg.Identity.PeerID
	if cid == "" {
		return fx.Error(errors.New("identity was not set in config (was 'ipfs init' run?)"))
	}
	if len(cid) == 0 {
		return fx.Error(errors.New("no peer ID in config! (was 'ipfs init' run?)"))
	}

	id, err := peer.Decode(cid)
	if err != nil {
		return fx.Error(fmt.Errorf("peer ID invalid: %s", err))
	}

	// Private Key

	if cfg.Identity.PrivKey == "" {
		return fx.Options( // No PK (usually in tests)
			fx.Provide(PeerID(id)),
			fx.Provide(libp2p.Peerstore),
		)
	}

	sk, err := cfg.Identity.DecodePrivateKey("passphrase todo!")
	if err != nil {
		return fx.Error(err)
	}

	return fx.Options( // Full identity
		fx.Provide(PeerID(id)),
		fx.Provide(PrivateKey(sk)),
		fx.Provide(libp2p.Peerstore),

		fx.Invoke(libp2p.PstoreAddSelfKeys),
	)
}

// IPNS groups namesys related units
var IPNS = fx.Options(
	fx.Provide(RecordValidator),
)

// Online groups online-only units
func Online(bcfg *BuildCfg, cfg *config.Config, userResourceOverrides rcmgr.PartialLimitConfig) fx.Option {
	// Namesys params

	ipnsCacheSize := cfg.Ipns.ResolveCacheSize
	if ipnsCacheSize == 0 {
		ipnsCacheSize = DefaultIpnsCacheSize
	}
	if ipnsCacheSize < 0 {
		return fx.Error(errors.New("cannot specify negative resolve cache size"))
	}

	// Republisher params

	var repubPeriod, recordLifetime time.Duration

	if cfg.Ipns.RepublishPeriod != "" {
		d, err := time.ParseDuration(cfg.Ipns.RepublishPeriod)
		if err != nil {
			return fx.Error(fmt.Errorf("failure to parse config setting IPNS.RepublishPeriod: %s", err))
		}

		if !util.Debug && (d < time.Minute || d > (time.Hour*24)) {
			return fx.Error(fmt.Errorf("config setting IPNS.RepublishPeriod is not between 1min and 1day: %s", d))
		}

		repubPeriod = d
	}

	if cfg.Ipns.RecordLifetime != "" {
		d, err := time.ParseDuration(cfg.Ipns.RecordLifetime)
		if err != nil {
			return fx.Error(fmt.Errorf("failure to parse config setting IPNS.RecordLifetime: %s", err))
		}

		recordLifetime = d
	}

	/* don't provide from bitswap when the strategic provider service is active */
	shouldBitswapProvide := !cfg.Experimental.StrategicProviding

	return fx.Options(
		fx.Provide(BitswapOptions(cfg)),
		fx.Provide(Bitswap(shouldBitswapProvide)),
		fx.Provide(OnlineExchange()),
		// Replace our Exchange with a Providing exchange!
		fx.Decorate(ProvidingExchange(shouldBitswapProvide)),
		fx.Provide(DNSResolver),
		fx.Provide(Namesys(ipnsCacheSize, cfg.Ipns.MaxCacheTTL.WithDefault(config.DefaultIpnsMaxCacheTTL))),
		fx.Provide(Peering),
		PeerWith(cfg.Peering.Peers...),

		fx.Invoke(IpnsRepublisher(repubPeriod, recordLifetime)),

		fx.Provide(p2p.New),

		LibP2P(bcfg, cfg, userResourceOverrides),
		OnlineProviders(
			cfg.Experimental.StrategicProviding,
			cfg.Reprovider.Strategy.WithDefault(config.DefaultReproviderStrategy),
			cfg.Reprovider.Interval.WithDefault(config.DefaultReproviderInterval),
			cfg.Routing.AcceleratedDHTClient.WithDefault(config.DefaultAcceleratedDHTClient),
		),
	)
}

// Offline groups offline alternatives to Online units
func Offline(cfg *config.Config) fx.Option {
	return fx.Options(
		fx.Provide(offline.Exchange),
		fx.Provide(DNSResolver),
		fx.Provide(Namesys(0, 0)),
		fx.Provide(libp2p.Routing),
		fx.Provide(libp2p.ContentRouting),
		fx.Provide(libp2p.OfflineRouting),
		OfflineProviders(),
	)
}

// Core groups basic IPFS services
var Core = fx.Options(
	fx.Provide(Dag),
	fx.Provide(FetcherConfig),
	fx.Provide(PathResolverConfig),
	fx.Provide(Pinning),
	fx.Provide(Files),
)

func Networked(bcfg *BuildCfg, cfg *config.Config, userResourceOverrides rcmgr.PartialLimitConfig) fx.Option {
	if bcfg.Online {
		return Online(bcfg, cfg, userResourceOverrides)
	}
	return Offline(cfg)
}

// IPFS builds a group of fx Options based on the passed BuildCfg
func IPFS(ctx context.Context, bcfg *BuildCfg) fx.Option {
	if bcfg == nil {
		bcfg = new(BuildCfg)
	}

	bcfgOpts, cfg := bcfg.options(ctx)
	if cfg == nil {
		return bcfgOpts // error
	}

	userResourceOverrides, err := bcfg.Repo.UserResourceOverrides()
	if err != nil {
		return fx.Error(err)
	}

	// Auto-sharding settings
	shardSizeString := cfg.Internal.UnixFSShardingSizeThreshold.WithDefault("256kiB")
	shardSizeInt, err := humanize.ParseBytes(shardSizeString)
	if err != nil {
		return fx.Error(err)
	}
	uio.HAMTShardingSize = int(shardSizeInt)

	// Migrate users of deprecated Experimental.ShardingEnabled flag
	if cfg.Experimental.ShardingEnabled {
		logger.Fatal("The `Experimental.ShardingEnabled` field is no longer used, please remove it from the config.\n" +
			"go-ipfs now automatically shards when directory block is bigger than  `" + shardSizeString + "`.\n" +
			"If you need to restore the old behavior (sharding everything) set `Internal.UnixFSShardingSizeThreshold` to `1B`.\n")
	}

	return fx.Options(
		bcfgOpts,

		fx.Provide(baseProcess),

		Storage(bcfg, cfg),
		Identity(cfg),
		IPNS,
		Networked(bcfg, cfg, userResourceOverrides),
		fx.Provide(BlockService(cfg)),
		Core,
	)
}
