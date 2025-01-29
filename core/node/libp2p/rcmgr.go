package libp2p

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/node/helpers"
	"github.com/ipfs/kubo/repo"

	"github.com/filecoin-project/go-clock"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/fx"
)

var rcmgrLogger = logging.Logger("rcmgr")

const NetLimitTraceFilename = "rcmgr.json.gz"

var ErrNoResourceMgr = errors.New("missing ResourceMgr: make sure the daemon is running with Swarm.ResourceMgr.Enabled")

func ResourceManager(repoPath string, cfg config.SwarmConfig, userResourceOverrides rcmgr.PartialLimitConfig) interface{} {
	return func(mctx helpers.MetricsCtx, lc fx.Lifecycle, repo repo.Repo) (network.ResourceManager, Libp2pOpts, error) {
		var manager network.ResourceManager
		var opts Libp2pOpts

		enabled := cfg.ResourceMgr.Enabled.WithDefault(true)

		//  ENV overrides Config (if present)
		switch os.Getenv("LIBP2P_RCMGR") {
		case "0", "false":
			enabled = false
		case "1", "true":
			enabled = true
		}

		if enabled {
			log.Debug("libp2p resource manager is enabled")

			limitConfig, msg, err := LimitConfig(cfg, userResourceOverrides)
			if err != nil {
				return nil, opts, fmt.Errorf("creating final Resource Manager config: %w", err)
			}

			if !isPartialConfigEmpty(userResourceOverrides) {
				rcmgrLogger.Info(`
libp2p-resource-limit-overrides.json has been loaded, "default" fields will be
filled in with autocomputed defaults.`)
			}

			// We want to see this message on startup, that's why we are using fmt instead of log.
			rcmgrLogger.Info(msg)

			if err := ensureConnMgrMakeSenseVsResourceMgr(limitConfig, cfg); err != nil {
				return nil, opts, err
			}

			str, err := rcmgr.NewStatsTraceReporter()
			if err != nil {
				return nil, opts, err
			}

			ropts := []rcmgr.Option{
				rcmgr.WithTraceReporter(str),
				rcmgr.WithLimitPerSubnet(
					nil,
					[]rcmgr.ConnLimitPerSubnet{
						{
							ConnCount:    16,
							PrefixLength: 56,
						},
						{
							ConnCount:    8 * 16,
							PrefixLength: 48,
						},
					}),
			}

			if len(cfg.ResourceMgr.Allowlist) > 0 {
				var mas []multiaddr.Multiaddr
				for _, maStr := range cfg.ResourceMgr.Allowlist {
					ma, err := multiaddr.NewMultiaddr(maStr)
					if err != nil {
						log.Errorf("failed to parse multiaddr=%v for allowlist, skipping. err=%v", maStr, err)
						continue
					}
					mas = append(mas, ma)
				}
				ropts = append(ropts, rcmgr.WithAllowlistedMultiaddrs(mas))
				log.Infof("Setting allowlist to: %v", mas)
			}

			if os.Getenv("LIBP2P_DEBUG_RCMGR") != "" {
				traceFilePath := filepath.Join(repoPath, NetLimitTraceFilename)
				ropts = append(ropts, rcmgr.WithTrace(traceFilePath))
			}

			limiter := rcmgr.NewFixedLimiter(limitConfig)

			manager, err = rcmgr.NewResourceManager(limiter, ropts...)
			if err != nil {
				return nil, opts, fmt.Errorf("creating libp2p resource manager: %w", err)
			}
			lrm := &loggingResourceManager{
				clock:    clock.New(),
				logger:   &logging.Logger("resourcemanager").SugaredLogger,
				delegate: manager,
			}
			lrm.start(helpers.LifecycleCtx(mctx, lc))
			manager = lrm
		} else {
			rcmgrLogger.Info("go-libp2p resource manager protection disabled")
			manager = &network.NullResourceManager{}
		}

		opts.Opts = append(opts.Opts, libp2p.ResourceManager(manager))

		lc.Append(fx.Hook{
			OnStop: func(_ context.Context) error {
				return manager.Close()
			},
		})

		return manager, opts, nil
	}
}

func isPartialConfigEmpty(cfg rcmgr.PartialLimitConfig) bool {
	var emptyResourceConfig rcmgr.ResourceLimits
	if cfg.System != emptyResourceConfig ||
		cfg.Transient != emptyResourceConfig ||
		cfg.AllowlistedSystem != emptyResourceConfig ||
		cfg.AllowlistedTransient != emptyResourceConfig ||
		cfg.ServiceDefault != emptyResourceConfig ||
		cfg.ServicePeerDefault != emptyResourceConfig ||
		cfg.ProtocolDefault != emptyResourceConfig ||
		cfg.ProtocolPeerDefault != emptyResourceConfig ||
		cfg.PeerDefault != emptyResourceConfig ||
		cfg.Conn != emptyResourceConfig ||
		cfg.Stream != emptyResourceConfig {
		return false
	}
	for _, v := range cfg.Service {
		if v != emptyResourceConfig {
			return false
		}
	}
	for _, v := range cfg.ServicePeer {
		if v != emptyResourceConfig {
			return false
		}
	}
	for _, v := range cfg.Protocol {
		if v != emptyResourceConfig {
			return false
		}
	}
	for _, v := range cfg.ProtocolPeer {
		if v != emptyResourceConfig {
			return false
		}
	}
	for _, v := range cfg.Peer {
		if v != emptyResourceConfig {
			return false
		}
	}
	return true
}

// LimitConfig returns the union of the Computed Default Limits and the User Supplied Override Limits.
func LimitConfig(cfg config.SwarmConfig, userResourceOverrides rcmgr.PartialLimitConfig) (limitConfig rcmgr.ConcreteLimitConfig, logMessageForStartup string, err error) {
	limitConfig, msg, err := createDefaultLimitConfig(cfg)
	if err != nil {
		return rcmgr.ConcreteLimitConfig{}, msg, err
	}

	// The logic for defaults and overriding with specified userResourceOverrides
	// is documented in docs/libp2p-resource-management.md.
	// Any changes here should be reflected there.

	// This effectively overrides the computed default LimitConfig with any non-"useDefault" values from the userResourceOverrides file.
	// Because of how how Build works, any rcmgr.Default value in userResourceOverrides
	// will be overridden with a computed default value.
	limitConfig = userResourceOverrides.Build(limitConfig)

	return limitConfig, msg, nil
}

type ResourceLimitsAndUsage struct {
	// This is duplicated from rcmgr.ResourceResourceLimits but adding *Usage fields.
	Memory               rcmgr.LimitVal64
	MemoryUsage          int64
	FD                   rcmgr.LimitVal
	FDUsage              int
	Conns                rcmgr.LimitVal
	ConnsUsage           int
	ConnsInbound         rcmgr.LimitVal
	ConnsInboundUsage    int
	ConnsOutbound        rcmgr.LimitVal
	ConnsOutboundUsage   int
	Streams              rcmgr.LimitVal
	StreamsUsage         int
	StreamsInbound       rcmgr.LimitVal
	StreamsInboundUsage  int
	StreamsOutbound      rcmgr.LimitVal
	StreamsOutboundUsage int
}

func (u ResourceLimitsAndUsage) ToResourceLimits() rcmgr.ResourceLimits {
	return rcmgr.ResourceLimits{
		Memory:          u.Memory,
		FD:              u.FD,
		Conns:           u.Conns,
		ConnsInbound:    u.ConnsInbound,
		ConnsOutbound:   u.ConnsOutbound,
		Streams:         u.Streams,
		StreamsInbound:  u.StreamsInbound,
		StreamsOutbound: u.StreamsOutbound,
	}
}

type LimitsConfigAndUsage struct {
	// This is duplicated from rcmgr.ResourceManagerStat but using ResourceLimitsAndUsage
	// instead of network.ScopeStat.
	System    ResourceLimitsAndUsage                 `json:",omitempty"`
	Transient ResourceLimitsAndUsage                 `json:",omitempty"`
	Services  map[string]ResourceLimitsAndUsage      `json:",omitempty"`
	Protocols map[protocol.ID]ResourceLimitsAndUsage `json:",omitempty"`
	Peers     map[peer.ID]ResourceLimitsAndUsage     `json:",omitempty"`
}

func (u LimitsConfigAndUsage) MarshalJSON() ([]byte, error) {
	// we want to marshal the encoded peer id
	encodedPeerMap := make(map[string]ResourceLimitsAndUsage, len(u.Peers))
	for p, v := range u.Peers {
		encodedPeerMap[p.String()] = v
	}

	type Alias LimitsConfigAndUsage
	return json.Marshal(&struct {
		*Alias
		Peers map[string]ResourceLimitsAndUsage `json:",omitempty"`
	}{
		Alias: (*Alias)(&u),
		Peers: encodedPeerMap,
	})
}

func (u LimitsConfigAndUsage) ToPartialLimitConfig() (result rcmgr.PartialLimitConfig) {
	result.System = u.System.ToResourceLimits()
	result.Transient = u.Transient.ToResourceLimits()

	result.Service = make(map[string]rcmgr.ResourceLimits, len(u.Services))
	for s, l := range u.Services {
		result.Service[s] = l.ToResourceLimits()
	}
	result.Protocol = make(map[protocol.ID]rcmgr.ResourceLimits, len(u.Protocols))
	for p, l := range u.Protocols {
		result.Protocol[p] = l.ToResourceLimits()
	}
	result.Peer = make(map[peer.ID]rcmgr.ResourceLimits, len(u.Peers))
	for p, l := range u.Peers {
		result.Peer[p] = l.ToResourceLimits()
	}

	return
}

func MergeLimitsAndStatsIntoLimitsConfigAndUsage(l rcmgr.ConcreteLimitConfig, stats rcmgr.ResourceManagerStat) LimitsConfigAndUsage {
	limits := l.ToPartialLimitConfig()

	return LimitsConfigAndUsage{
		System:    mergeResourceLimitsAndScopeStatToResourceLimitsAndUsage(limits.System, stats.System),
		Transient: mergeResourceLimitsAndScopeStatToResourceLimitsAndUsage(limits.Transient, stats.Transient),
		Services:  mergeLimitsAndStatsMapIntoLimitsConfigAndUsageMap(limits.Service, stats.Services),
		Protocols: mergeLimitsAndStatsMapIntoLimitsConfigAndUsageMap(limits.Protocol, stats.Protocols),
		Peers:     mergeLimitsAndStatsMapIntoLimitsConfigAndUsageMap(limits.Peer, stats.Peers),
	}
}

func mergeLimitsAndStatsMapIntoLimitsConfigAndUsageMap[K comparable](limits map[K]rcmgr.ResourceLimits, stats map[K]network.ScopeStat) map[K]ResourceLimitsAndUsage {
	r := make(map[K]ResourceLimitsAndUsage, maxInt(len(limits), len(stats)))
	for p, s := range stats {
		var l rcmgr.ResourceLimits
		if limits != nil {
			if rl, ok := limits[p]; ok {
				l = rl
			}
		}
		r[p] = mergeResourceLimitsAndScopeStatToResourceLimitsAndUsage(l, s)
	}
	for p, s := range limits {
		if _, ok := stats[p]; ok {
			continue // we already processed this element in the loop above
		}

		r[p] = mergeResourceLimitsAndScopeStatToResourceLimitsAndUsage(s, network.ScopeStat{})
	}
	return r
}

func maxInt(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func mergeResourceLimitsAndScopeStatToResourceLimitsAndUsage(rl rcmgr.ResourceLimits, ss network.ScopeStat) ResourceLimitsAndUsage {
	return ResourceLimitsAndUsage{
		Memory:               rl.Memory,
		MemoryUsage:          ss.Memory,
		FD:                   rl.FD,
		FDUsage:              ss.NumFD,
		Conns:                rl.Conns,
		ConnsUsage:           ss.NumConnsOutbound + ss.NumConnsInbound,
		ConnsOutbound:        rl.ConnsOutbound,
		ConnsOutboundUsage:   ss.NumConnsOutbound,
		ConnsInbound:         rl.ConnsInbound,
		ConnsInboundUsage:    ss.NumConnsInbound,
		Streams:              rl.Streams,
		StreamsUsage:         ss.NumStreamsOutbound + ss.NumStreamsInbound,
		StreamsOutbound:      rl.StreamsOutbound,
		StreamsOutboundUsage: ss.NumStreamsOutbound,
		StreamsInbound:       rl.StreamsInbound,
		StreamsInboundUsage:  ss.NumStreamsInbound,
	}
}

type ResourceInfos []ResourceInfo

type ResourceInfo struct {
	ScopeName    string
	LimitName    string
	LimitValue   rcmgr.LimitVal64
	CurrentUsage int64
}

// LimitConfigsToInfo gets limits and stats and generates a list of scopes and limits to be printed.
func LimitConfigsToInfo(stats LimitsConfigAndUsage) ResourceInfos {
	result := ResourceInfos{}

	result = append(result, resourceLimitsAndUsageToResourceInfo(config.ResourceMgrSystemScope, stats.System)...)
	result = append(result, resourceLimitsAndUsageToResourceInfo(config.ResourceMgrTransientScope, stats.Transient)...)

	for i, s := range stats.Services {
		result = append(result, resourceLimitsAndUsageToResourceInfo(
			config.ResourceMgrServiceScopePrefix+i,
			s,
		)...)
	}

	for i, p := range stats.Protocols {
		result = append(result, resourceLimitsAndUsageToResourceInfo(
			config.ResourceMgrProtocolScopePrefix+string(i),
			p,
		)...)
	}

	for i, p := range stats.Peers {
		result = append(result, resourceLimitsAndUsageToResourceInfo(
			config.ResourceMgrPeerScopePrefix+i.String(),
			p,
		)...)
	}

	return result
}

const (
	limitNameMemory          = "Memory"
	limitNameFD              = "FD"
	limitNameConns           = "Conns"
	limitNameConnsInbound    = "ConnsInbound"
	limitNameConnsOutbound   = "ConnsOutbound"
	limitNameStreams         = "Streams"
	limitNameStreamsInbound  = "StreamsInbound"
	limitNameStreamsOutbound = "StreamsOutbound"
)

var limits = []string{
	limitNameMemory,
	limitNameFD,
	limitNameConns,
	limitNameConnsInbound,
	limitNameConnsOutbound,
	limitNameStreams,
	limitNameStreamsInbound,
	limitNameStreamsOutbound,
}

func resourceLimitsAndUsageToResourceInfo(scopeName string, stats ResourceLimitsAndUsage) ResourceInfos {
	result := ResourceInfos{}
	for _, l := range limits {
		ri := ResourceInfo{
			ScopeName: scopeName,
		}
		switch l {
		case limitNameMemory:
			ri.LimitName = limitNameMemory
			ri.LimitValue = stats.Memory
			ri.CurrentUsage = stats.MemoryUsage
		case limitNameFD:
			ri.LimitName = limitNameFD
			ri.LimitValue = rcmgr.LimitVal64(stats.FD)
			ri.CurrentUsage = int64(stats.FDUsage)
		case limitNameConns:
			ri.LimitName = limitNameConns
			ri.LimitValue = rcmgr.LimitVal64(stats.Conns)
			ri.CurrentUsage = int64(stats.ConnsUsage)
		case limitNameConnsInbound:
			ri.LimitName = limitNameConnsInbound
			ri.LimitValue = rcmgr.LimitVal64(stats.ConnsInbound)
			ri.CurrentUsage = int64(stats.ConnsInboundUsage)
		case limitNameConnsOutbound:
			ri.LimitName = limitNameConnsOutbound
			ri.LimitValue = rcmgr.LimitVal64(stats.ConnsOutbound)
			ri.CurrentUsage = int64(stats.ConnsOutboundUsage)
		case limitNameStreams:
			ri.LimitName = limitNameStreams
			ri.LimitValue = rcmgr.LimitVal64(stats.Streams)
			ri.CurrentUsage = int64(stats.StreamsUsage)
		case limitNameStreamsInbound:
			ri.LimitName = limitNameStreamsInbound
			ri.LimitValue = rcmgr.LimitVal64(stats.StreamsInbound)
			ri.CurrentUsage = int64(stats.StreamsInboundUsage)
		case limitNameStreamsOutbound:
			ri.LimitName = limitNameStreamsOutbound
			ri.LimitValue = rcmgr.LimitVal64(stats.StreamsOutbound)
			ri.CurrentUsage = int64(stats.StreamsOutboundUsage)
		}

		if ri.LimitValue == rcmgr.Unlimited64 || ri.LimitValue == rcmgr.DefaultLimit64 {
			// ignore unlimited and unset limits to remove noise from output.
			continue
		}

		result = append(result, ri)
	}

	return result
}

func ensureConnMgrMakeSenseVsResourceMgr(concreteLimits rcmgr.ConcreteLimitConfig, cfg config.SwarmConfig) error {
	if cfg.ConnMgr.Type.WithDefault(config.DefaultConnMgrType) == "none" || len(cfg.ResourceMgr.Allowlist) != 0 {
		// no connmgr OR
		// If an allowlist is set, a user may be enacting some form of DoS defense.
		// We don't want want to modify the System.ConnsInbound in that case for example
		// as it may make sense for it to be (and stay) as "blockAll"
		// so that only connections within the allowlist of multiaddrs get established.
		return nil
	}

	rcm := concreteLimits.ToPartialLimitConfig()

	highWater := cfg.ConnMgr.HighWater.WithDefault(config.DefaultConnMgrHighWater)
	if (rcm.System.Conns > rcmgr.DefaultLimit || rcm.System.Conns == rcmgr.BlockAllLimit) && int64(rcm.System.Conns) <= highWater {
		// nolint
		return fmt.Errorf(`
Unable to initialize libp2p due to conflicting resource manager limit configuration.
resource manager System.Conns (%d) must be bigger than ConnMgr.HighWater (%d)
See: https://github.com/ipfs/kubo/blob/master/docs/libp2p-resource-management.md#how-does-the-resource-manager-resourcemgr-relate-to-the-connection-manager-connmgr
`, rcm.System.Conns, highWater)
	}
	if (rcm.System.ConnsInbound > rcmgr.DefaultLimit || rcm.System.ConnsInbound == rcmgr.BlockAllLimit) && int64(rcm.System.ConnsInbound) <= highWater {
		// nolint
		return fmt.Errorf(`
Unable to initialize libp2p due to conflicting resource manager limit configuration.
resource manager System.ConnsInbound (%d) must be bigger than ConnMgr.HighWater (%d)
See: https://github.com/ipfs/kubo/blob/master/docs/libp2p-resource-management.md#how-does-the-resource-manager-resourcemgr-relate-to-the-connection-manager-connmgr
`, rcm.System.ConnsInbound, highWater)
	}
	if (rcm.System.Streams > rcmgr.DefaultLimit || rcm.System.Streams == rcmgr.BlockAllLimit) && int64(rcm.System.Streams) <= highWater {
		// nolint
		return fmt.Errorf(`
Unable to initialize libp2p due to conflicting resource manager limit configuration.
resource manager System.Streams (%d) must be bigger than ConnMgr.HighWater (%d)
See: https://github.com/ipfs/kubo/blob/master/docs/libp2p-resource-management.md#how-does-the-resource-manager-resourcemgr-relate-to-the-connection-manager-connmgr
`, rcm.System.Streams, highWater)
	}
	if (rcm.System.StreamsInbound > rcmgr.DefaultLimit || rcm.System.StreamsInbound == rcmgr.BlockAllLimit) && int64(rcm.System.StreamsInbound) <= highWater {
		// nolint
		return fmt.Errorf(`
Unable to initialize libp2p due to conflicting resource manager limit configuration.
resource manager System.StreamsInbound (%d) must be bigger than ConnMgr.HighWater (%d)
See: https://github.com/ipfs/kubo/blob/master/docs/libp2p-resource-management.md#how-does-the-resource-manager-resourcemgr-relate-to-the-connection-manager-connmgr
`, rcm.System.StreamsInbound, highWater)
	}
	return nil
}
