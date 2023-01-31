package libp2p

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/benbjohnson/clock"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	rcmgrObs "github.com/libp2p/go-libp2p/p2p/host/resource-manager/obs"
	"github.com/multiformats/go-multiaddr"
	"go.opencensus.io/stats/view"
	"go.uber.org/fx"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/node/helpers"
	"github.com/ipfs/kubo/repo"
)

const NetLimitDefaultFilename = "limit.json"
const NetLimitTraceFilename = "rcmgr.json.gz"

var ErrNoResourceMgr = fmt.Errorf("missing ResourceMgr: make sure the daemon is running with Swarm.ResourceMgr.Enabled")

func ResourceManager(cfg config.SwarmConfig) interface{} {
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

			repoPath, err := config.PathRoot()
			if err != nil {
				return nil, opts, fmt.Errorf("opening IPFS_PATH: %w", err)
			}

			concreteLimitConfig, err := createDefaultLimitConfig(cfg, true)
			if err != nil {
				return nil, opts, err
			}

			if err := ensureConnMgrMakeSenseVsResourceMgr(concreteLimitConfig, cfg.ConnMgr); err != nil {
				return nil, opts, err
			}

			str, err := rcmgrObs.NewStatsTraceReporter()
			if err != nil {
				return nil, opts, err
			}

			ropts := []rcmgr.Option{rcmgr.WithMetrics(createRcmgrMetrics()), rcmgr.WithTraceReporter(str)}

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

			err = view.Register(rcmgrObs.DefaultViews...)
			if err != nil {
				return nil, opts, fmt.Errorf("registering rcmgr obs views: %w", err)
			}

			if os.Getenv("LIBP2P_DEBUG_RCMGR") != "" {
				traceFilePath := filepath.Join(repoPath, NetLimitTraceFilename)
				ropts = append(ropts, rcmgr.WithTrace(traceFilePath))
			}

			limiter := rcmgr.NewFixedLimiter(concreteLimitConfig)

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
			fmt.Println("go-libp2p resource manager protection disabled")
			manager = &network.NullResourceManager{}
		}

		opts.Opts = append(opts.Opts, libp2p.ResourceManager(manager))

		lc.Append(fx.Hook{
			OnStop: func(_ context.Context) error {
				return manager.Close()
			}})

		return manager, opts, nil
	}
}

type NetStatOut struct {
	System    *rcmgr.ResourceLimits           `json:",omitempty"`
	Transient *rcmgr.ResourceLimits           `json:",omitempty"`
	Services  map[string]rcmgr.ResourceLimits `json:",omitempty"`
	Protocols map[string]rcmgr.ResourceLimits `json:",omitempty"`
	Peers     map[string]rcmgr.ResourceLimits `json:",omitempty"`
}

func NetStat(mgr network.ResourceManager, scope string, percentage int) (NetStatOut, error) {
	var err error
	var result NetStatOut
	switch {
	case scope == "all":
		rapi, ok := mgr.(rcmgr.ResourceManagerState)
		if !ok { // NullResourceManager
			return result, ErrNoResourceMgr
		}

		limits, err := NetLimitAll(mgr)
		if err != nil {
			return result, err
		}

		stat := rapi.Stat()
		result.System = compareLimits(scopeToLimit(&stat.System), limits.System, percentage)
		result.Transient = compareLimits(scopeToLimit(&stat.Transient), limits.Transient, percentage)
		if len(stat.Services) > 0 {
			result.Services = make(map[string]rcmgr.ResourceLimits, len(stat.Services))
			for srv, stat := range stat.Services {
				ls := limits.Services[srv]
				fstat := compareLimits(scopeToLimit(&stat), &ls, percentage)
				if fstat != nil {
					result.Services[srv] = *fstat
				}
			}
		}
		if len(stat.Protocols) > 0 {
			result.Protocols = make(map[string]rcmgr.ResourceLimits, len(stat.Protocols))
			for proto, stat := range stat.Protocols {
				ls := limits.Protocols[string(proto)]
				fstat := compareLimits(scopeToLimit(&stat), &ls, percentage)
				if fstat != nil {
					result.Protocols[string(proto)] = *fstat
				}
			}
		}
		if len(stat.Peers) > 0 {
			result.Peers = make(map[string]rcmgr.ResourceLimits, len(stat.Peers))
			for p, stat := range stat.Peers {
				ls := limits.Peers[p.Pretty()]
				fstat := compareLimits(scopeToLimit(&stat), &ls, percentage)
				if fstat != nil {
					result.Peers[p.Pretty()] = *fstat
				}
			}
		}

		return result, nil

	case scope == config.ResourceMgrSystemScope:
		err = mgr.ViewSystem(func(s network.ResourceScope) error {
			stat := s.Stat()
			result.System = scopeToLimit(&stat)
			return nil
		})
		return result, err

	case scope == config.ResourceMgrTransientScope:
		err = mgr.ViewTransient(func(s network.ResourceScope) error {
			stat := s.Stat()
			result.Transient = scopeToLimit(&stat)
			return nil
		})
		return result, err

	case strings.HasPrefix(scope, config.ResourceMgrServiceScopePrefix):
		svc := strings.TrimPrefix(scope, config.ResourceMgrServiceScopePrefix)
		err = mgr.ViewService(svc, func(s network.ServiceScope) error {
			stat := s.Stat()
			result.Services = map[string]rcmgr.ResourceLimits{
				svc: *scopeToLimit(&stat),
			}
			return nil
		})
		return result, err

	case strings.HasPrefix(scope, config.ResourceMgrProtocolScopePrefix):
		proto := strings.TrimPrefix(scope, config.ResourceMgrProtocolScopePrefix)
		err = mgr.ViewProtocol(protocol.ID(proto), func(s network.ProtocolScope) error {
			stat := s.Stat()
			result.Protocols = map[string]rcmgr.ResourceLimits{
				proto: *scopeToLimit(&stat),
			}
			return nil
		})
		return result, err

	case strings.HasPrefix(scope, config.ResourceMgrPeerScopePrefix):
		p := strings.TrimPrefix(scope, config.ResourceMgrPeerScopePrefix)
		pid, err := peer.Decode(p)
		if err != nil {
			return result, fmt.Errorf("invalid peer ID: %q: %w", p, err)
		}
		err = mgr.ViewPeer(pid, func(s network.PeerScope) error {
			stat := s.Stat()
			result.Peers = map[string]rcmgr.ResourceLimits{
				p: *scopeToLimit(&stat),
			}
			return nil
		})
		return result, err

	default:
		return result, fmt.Errorf("invalid scope %q", scope)
	}
}

var scopes = []string{
	config.ResourceMgrSystemScope,
	config.ResourceMgrTransientScope,
	config.ResourceMgrServiceScopePrefix,
	config.ResourceMgrProtocolScopePrefix,
	config.ResourceMgrPeerScopePrefix,
}

func scopeToLimit(s *network.ScopeStat) *rcmgr.ResourceLimits {

	return &rcmgr.ResourceLimits{
		Streams:         rcmgr.LimitVal(s.NumStreamsInbound + s.NumStreamsOutbound),
		StreamsInbound:  rcmgr.LimitVal(s.NumStreamsInbound),
		StreamsOutbound: rcmgr.LimitVal(s.NumStreamsOutbound),
		Conns:           rcmgr.LimitVal(s.NumConnsInbound + s.NumConnsOutbound),
		ConnsInbound:    rcmgr.LimitVal(s.NumConnsInbound),
		ConnsOutbound:   rcmgr.LimitVal(s.NumConnsOutbound),
		FD:              rcmgr.LimitVal(s.NumFD),
		Memory:          rcmgr.LimitVal64(s.Memory),
	}
}

// compareLimits compares stat and limit.
// If any of the stats value are equals or above the specified percentage,
// stat object is returned.
func compareLimits(stat, limit *rcmgr.ResourceLimits, percentage int) *rcmgr.ResourceLimits {
	if stat == nil || limit == nil {
		return nil
	}
	if abovePercentage(int(stat.Memory), int(limit.Memory), percentage) {
		return stat
	}
	if abovePercentage(int(stat.ConnsInbound), int(limit.ConnsInbound), percentage) {
		return stat
	}
	if abovePercentage(int(stat.ConnsOutbound), int(limit.ConnsOutbound), percentage) {
		return stat
	}
	if abovePercentage(int(stat.Conns), int(limit.Conns), percentage) {
		return stat
	}
	if abovePercentage(int(stat.FD), int(limit.FD), percentage) {
		return stat
	}
	if abovePercentage(int(stat.StreamsInbound), int(limit.StreamsInbound), percentage) {
		return stat
	}
	if abovePercentage(int(stat.StreamsOutbound), int(limit.StreamsOutbound), percentage) {
		return stat
	}
	if abovePercentage(int(stat.Streams), int(limit.Streams), percentage) {
		return stat
	}

	return nil
}

func abovePercentage(v1, v2, percentage int) bool {
	if percentage == 0 {
		return true
	}

	if v2 == 0 {
		return false
	}

	return int((float64(v1)/float64(v2))*100) >= percentage
}

var infiniteBaseLimit = rcmgr.BaseLimit{
	Streams:         math.MaxInt,
	StreamsInbound:  math.MaxInt,
	StreamsOutbound: math.MaxInt,
	Conns:           math.MaxInt,
	ConnsInbound:    math.MaxInt,
	ConnsOutbound:   math.MaxInt,
	FD:              math.MaxInt,
	Memory:          math.MaxInt64,
}

func NetLimitAll(mgr network.ResourceManager) (*NetStatOut, error) {
	var result = &NetStatOut{}
	lister, ok := mgr.(rcmgr.ResourceManagerState)
	if !ok { // NullResourceManager
		return result, ErrNoResourceMgr
	}

	for _, s := range scopes {
		switch s {
		case config.ResourceMgrSystemScope:
			s, err := NetLimit(mgr, config.ResourceMgrSystemScope)
			if err != nil {
				return nil, err
			}
			result.System = s
		case config.ResourceMgrTransientScope:
			s, err := NetLimit(mgr, config.ResourceMgrSystemScope)
			if err != nil {
				return nil, err
			}
			result.Transient = s
		case config.ResourceMgrServiceScopePrefix:
			result.Services = make(map[string]rcmgr.ResourceLimits)
			for _, serv := range lister.ListServices() {
				s, err := NetLimit(mgr, config.ResourceMgrServiceScopePrefix+serv)
				if err != nil {
					return nil, err
				}
				result.Services[serv] = *s
			}
		case config.ResourceMgrProtocolScopePrefix:
			result.Protocols = make(map[string]rcmgr.ResourceLimits)
			for _, prot := range lister.ListProtocols() {
				ps := string(prot)
				s, err := NetLimit(mgr, config.ResourceMgrProtocolScopePrefix+ps)
				if err != nil {
					return nil, err
				}
				result.Protocols[ps] = *s
			}
		case config.ResourceMgrPeerScopePrefix:
			result.Peers = make(map[string]rcmgr.ResourceLimits)
			for _, peer := range lister.ListPeers() {
				ps := peer.Pretty()
				s, err := NetLimit(mgr, config.ResourceMgrPeerScopePrefix+ps)
				if err != nil {
					return nil, err
				}
				result.Peers[ps] = *s
			}
		}
	}

	return result, nil
}

func NetLimit(mgr network.ResourceManager, scope string) (*rcmgr.ResourceLimits, error) {
	var result *rcmgr.ResourceLimits
	getLimit := func(s network.ResourceScope) error {
		limiter, ok := s.(rcmgr.ResourceScopeLimiter)
		if !ok { // NullResourceManager
			return ErrNoResourceMgr
		}
		limit := limiter.Limit()
		switch l := limit.(type) {
		case *rcmgr.BaseLimit:
			result = l.ToResourceLimits()
		default:
			return fmt.Errorf("unknown limit type %T", limit)
		}

		return nil
	}

	switch {
	case scope == config.ResourceMgrSystemScope:
		return result, mgr.ViewSystem(func(s network.ResourceScope) error { return getLimit(s) })
	case scope == config.ResourceMgrTransientScope:
		return result, mgr.ViewTransient(func(s network.ResourceScope) error { return getLimit(s) })
	case strings.HasPrefix(scope, config.ResourceMgrServiceScopePrefix):
		svc := strings.TrimPrefix(scope, config.ResourceMgrServiceScopePrefix)
		return result, mgr.ViewService(svc, func(s network.ServiceScope) error { return getLimit(s) })
	case strings.HasPrefix(scope, config.ResourceMgrProtocolScopePrefix):
		proto := strings.TrimPrefix(scope, config.ResourceMgrProtocolScopePrefix)
		return result, mgr.ViewProtocol(protocol.ID(proto), func(s network.ProtocolScope) error { return getLimit(s) })
	case strings.HasPrefix(scope, config.ResourceMgrPeerScopePrefix):
		p := strings.TrimPrefix(scope, config.ResourceMgrPeerScopePrefix)
		pid, err := peer.Decode(p)
		if err != nil {
			return result, fmt.Errorf("invalid peer ID: %q: %w", p, err)
		}
		return result, mgr.ViewPeer(pid, func(s network.PeerScope) error { return getLimit(s) })
	default:
		return result, fmt.Errorf("invalid scope %q", scope)
	}
}

// NetSetLimit sets new ResourceManager limits for the given scope. The limits take effect immediately, and are also persisted to the repo config.
func NetSetLimit(mgr network.ResourceManager, repo repo.Repo, scope string, limit *rcmgr.ResourceLimits) error {
	setLimit := func(s network.ResourceScope) error {
		limiter, ok := s.(rcmgr.ResourceScopeLimiter)
		if !ok { // NullResourceManager
			return ErrNoResourceMgr
		}

		baseLimit := limit.Build(infiniteBaseLimit)
		limiter.SetLimit(&baseLimit)
		return nil
	}

	cfg, err := repo.Config()
	if err != nil {
		return fmt.Errorf("reading config to set limit: %w", err)
	}

	if cfg.Swarm.ResourceMgr.Limits == nil {
		cfg.Swarm.ResourceMgr.Limits = &rcmgr.PartialLimitConfig{}
	}
	configLimits := cfg.Swarm.ResourceMgr.Limits

	var setConfigFunc func()
	switch {
	case scope == config.ResourceMgrSystemScope:
		err = mgr.ViewSystem(func(s network.ResourceScope) error { return setLimit(s) })
		setConfigFunc = func() { configLimits.System = limit }
	case scope == config.ResourceMgrTransientScope:
		err = mgr.ViewTransient(func(s network.ResourceScope) error { return setLimit(s) })
		setConfigFunc = func() { configLimits.Transient = limit }
	case strings.HasPrefix(scope, config.ResourceMgrServiceScopePrefix):
		svc := strings.TrimPrefix(scope, config.ResourceMgrServiceScopePrefix)
		err = mgr.ViewService(svc, func(s network.ServiceScope) error { return setLimit(s) })
		setConfigFunc = func() {
			if configLimits.Service == nil {
				configLimits.Service = map[string]rcmgr.ResourceLimits{}
			}
			configLimits.Service[svc] = *limit
		}
	case strings.HasPrefix(scope, config.ResourceMgrProtocolScopePrefix):
		proto := strings.TrimPrefix(scope, config.ResourceMgrProtocolScopePrefix)
		err = mgr.ViewProtocol(protocol.ID(proto), func(s network.ProtocolScope) error { return setLimit(s) })
		setConfigFunc = func() {
			if configLimits.Protocol == nil {
				configLimits.Protocol = map[protocol.ID]rcmgr.ResourceLimits{}
			}
			configLimits.Protocol[protocol.ID(proto)] = *limit
		}
	case strings.HasPrefix(scope, config.ResourceMgrPeerScopePrefix):
		p := strings.TrimPrefix(scope, config.ResourceMgrPeerScopePrefix)
		var pid peer.ID
		pid, err = peer.Decode(p)
		if err != nil {
			return fmt.Errorf("invalid peer ID: %q: %w", p, err)
		}
		err = mgr.ViewPeer(pid, func(s network.PeerScope) error { return setLimit(s) })
		setConfigFunc = func() {
			if configLimits.Peer == nil {
				configLimits.Peer = map[peer.ID]rcmgr.ResourceLimits{}
			}
			configLimits.Peer[pid] = *limit
		}
	default:
		return fmt.Errorf("invalid scope %q", scope)
	}

	if err != nil {
		return fmt.Errorf("setting new limits on resource manager: %w", err)
	}

	if cfg.Swarm.ResourceMgr.Limits == nil {
		cfg.Swarm.ResourceMgr.Limits = &rcmgr.PartialLimitConfig{}
	}
	setConfigFunc()

	if err := repo.SetConfig(cfg); err != nil {
		return fmt.Errorf("writing new limits to repo config: %w", err)
	}

	return nil
}

// NetResetLimit resets ResourceManager limits to defaults. The limits take effect immediately, and are also persisted to the repo config.
func NetResetLimit(mgr network.ResourceManager, repo repo.Repo, scope string) (*rcmgr.ResourceLimits, error) {
	var result *rcmgr.ResourceLimits

	setLimit := func(s network.ResourceScope, limit *rcmgr.ResourceLimits) error {
		limiter, ok := s.(rcmgr.ResourceScopeLimiter)
		if !ok {
			return ErrNoResourceMgr
		}

		baseLimit := limit.Build(infiniteBaseLimit)
		result = limit
		limiter.SetLimit(&baseLimit)
		return nil
	}

	cfg, err := repo.Config()
	if err != nil {
		return result, fmt.Errorf("reading config to reset limit: %w", err)
	}

	clc, err := createDefaultLimitConfig(cfg.Swarm, false)
	if err != nil {
		return result, fmt.Errorf("creating default limit config: %w", err)
	}

	defaults := clc.ToLimitConfig()

	if cfg.Swarm.ResourceMgr.Limits == nil {
		cfg.Swarm.ResourceMgr.Limits = &rcmgr.PartialLimitConfig{}
	}
	configLimits := cfg.Swarm.ResourceMgr.Limits

	switch {
	case scope == config.ResourceMgrSystemScope:
		err = mgr.ViewSystem(func(s network.ResourceScope) error { return setLimit(s, defaults.System) })
		configLimits.System = nil
	case scope == config.ResourceMgrTransientScope:
		err = mgr.ViewTransient(func(s network.ResourceScope) error { return setLimit(s, defaults.Transient) })
		configLimits.Transient = nil
	case strings.HasPrefix(scope, config.ResourceMgrServiceScopePrefix):
		svc := strings.TrimPrefix(scope, config.ResourceMgrServiceScopePrefix)

		err = mgr.ViewService(svc, func(s network.ServiceScope) error { return setLimit(s, defaults.ServiceDefault) })
		if configLimits.Service == nil {
			configLimits.Service = map[string]rcmgr.ResourceLimits{}
		}

		delete(configLimits.Service, svc)
	case strings.HasPrefix(scope, config.ResourceMgrProtocolScopePrefix):
		proto := strings.TrimPrefix(scope, config.ResourceMgrProtocolScopePrefix)

		err = mgr.ViewProtocol(protocol.ID(proto), func(s network.ProtocolScope) error { return setLimit(s, defaults.ProtocolDefault) })
		if configLimits.Protocol == nil {
			configLimits.Protocol = map[protocol.ID]rcmgr.ResourceLimits{}
		}

		delete(configLimits.Protocol, protocol.ID(proto))
	case strings.HasPrefix(scope, config.ResourceMgrPeerScopePrefix):
		p := strings.TrimPrefix(scope, config.ResourceMgrPeerScopePrefix)

		var pid peer.ID
		pid, err = peer.Decode(p)
		if err != nil {
			return result, fmt.Errorf("invalid peer ID: %q: %w", p, err)
		}

		err = mgr.ViewPeer(pid, func(s network.PeerScope) error { return setLimit(s, defaults.PeerDefault) })
		if configLimits.Peer == nil {
			configLimits.Peer = map[peer.ID]rcmgr.ResourceLimits{}
		}

		delete(configLimits.Peer, pid)
	default:
		return result, fmt.Errorf("invalid scope %q", scope)
	}

	if err != nil {
		return result, fmt.Errorf("resetting new limits on resource manager: %w", err)
	}

	if err := repo.SetConfig(cfg); err != nil {
		return result, fmt.Errorf("writing new limits to repo config: %w", err)
	}

	return result, nil
}

func ensureConnMgrMakeSenseVsResourceMgr(concreteLimits rcmgr.ConcreteLimitConfig, cmgr config.ConnMgr) error {
	if cmgr.Type.WithDefault(config.DefaultConnMgrType) == "none" {
		return nil // none connmgr, no checks to do
	}

	rcm := concreteLimits.ToLimitConfig()

	highWater := cmgr.HighWater.WithDefault(config.DefaultConnMgrHighWater)
	if rcm.System.ConnsInbound <= rcm.System.Conns {
		if int64(rcm.System.ConnsInbound) <= highWater {
			// nolint
			return fmt.Errorf(`
Unable to initialize libp2p due to conflicting limit configuration:
ResourceMgr.Limits.System.ConnsInbound (%d) must be bigger than ConnMgr.HighWater (%d)
`, rcm.System.ConnsInbound, highWater)
		}
	} else if int64(rcm.System.Conns) <= highWater {
		// nolint
		return fmt.Errorf(`
Unable to initialize libp2p due to conflicting limit configuration:
ResourceMgr.Limits.System.Conns (%d) must be bigger than ConnMgr.HighWater (%d)
`, rcm.System.Conns, highWater)
	}
	if rcm.System.StreamsInbound <= rcm.System.Streams {
		if int64(rcm.System.StreamsInbound) <= highWater {
			// nolint
			return fmt.Errorf(`
Unable to initialize libp2p due to conflicting limit configuration:
ResourceMgr.Limits.System.StreamsInbound (%d) must be bigger than ConnMgr.HighWater (%d)
`, rcm.System.StreamsInbound, highWater)
		}
	} else if int64(rcm.System.Streams) <= highWater {
		// nolint
		return fmt.Errorf(`
Unable to initialize libp2p due to conflicting limit configuration:
ResourceMgr.Limits.System.Streams (%d) must be bigger than ConnMgr.HighWater (%d)
`, rcm.System.Streams, highWater)
	}
	return nil
}
