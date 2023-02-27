package libp2p

import (
	"context"
	"fmt"
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
	"go.uber.org/fx"
	"golang.org/x/exp/constraints"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/node/helpers"
	"github.com/ipfs/kubo/repo"
)

// FIXME(@Jorropo): for go-libp2p v0.26.0 use .MustConcrete and .MustBaseLimit instead of .Build(rcmgr.BaseLimit{}).

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

			var limitConfig rcmgr.ConcreteLimitConfig
			defaultComputedLimitConfig, err := createDefaultLimitConfig(cfg)
			if err != nil {
				return nil, opts, err
			}

			// The logic for defaults and overriding with specified SwarmConfig.ResourceMgr.Limits
			// is documented in docs/config.md.
			// Any changes here should be reflected there.
			if cfg.ResourceMgr.Limits != nil {
				userSuppliedOverrideLimitConfig := *cfg.ResourceMgr.Limits
				// This effectively overrides the computed default LimitConfig with any non-zero values from cfg.ResourceMgr.Limits.
				// Because of how how Apply works, any 0 value for a user supplied override
				// will be overriden with a computed default value.
				// There currently isn't a way for a user to supply a 0-value override.
				limitConfig = userSuppliedOverrideLimitConfig.Build(defaultComputedLimitConfig)
			} else {
				limitConfig = defaultComputedLimitConfig
			}

			if err := ensureConnMgrMakeSenseVsResourceMgr(limitConfig, cfg.ConnMgr); err != nil {
				return nil, opts, err
			}

			limiter := rcmgr.NewFixedLimiter(limitConfig)

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

			if os.Getenv("LIBP2P_DEBUG_RCMGR") != "" {
				traceFilePath := filepath.Join(repoPath, NetLimitTraceFilename)
				ropts = append(ropts, rcmgr.WithTrace(traceFilePath))
			}

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

type notOmitEmptyResourceLimit struct {
	Streams         rcmgr.LimitVal
	StreamsInbound  rcmgr.LimitVal
	StreamsOutbound rcmgr.LimitVal
	Conns           rcmgr.LimitVal
	ConnsInbound    rcmgr.LimitVal
	ConnsOutbound   rcmgr.LimitVal
	FD              rcmgr.LimitVal
	Memory          rcmgr.LimitVal64
}

func resourceLimitsToNotOmitEmpty(r rcmgr.ResourceLimits) notOmitEmptyResourceLimit {
	return notOmitEmptyResourceLimit{
		Streams:         r.Streams,
		StreamsInbound:  r.StreamsInbound,
		StreamsOutbound: r.StreamsOutbound,
		Conns:           r.Conns,
		ConnsInbound:    r.ConnsInbound,
		ConnsOutbound:   r.ConnsOutbound,
		FD:              r.FD,
		Memory:          r.Memory,
	}
}

type NetStatOut struct {
	System    *notOmitEmptyResourceLimit           `json:",omitempty"`
	Transient *notOmitEmptyResourceLimit           `json:",omitempty"`
	Services  map[string]notOmitEmptyResourceLimit `json:",omitempty"`
	Protocols map[string]notOmitEmptyResourceLimit `json:",omitempty"`
	Peers     map[string]notOmitEmptyResourceLimit `json:",omitempty"`
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
		if s := scopeToLimit(stat.System); compareLimits(s, *limits.System, percentage) {
			result.System = &s
		}
		if s := scopeToLimit(stat.Transient); compareLimits(s, *limits.Transient, percentage) {
			result.Transient = &s
		}
		if len(stat.Services) > 0 {
			result.Services = make(map[string]notOmitEmptyResourceLimit, len(stat.Services))
			for srv, s := range stat.Services {
				ls := limits.Services[srv]
				if stat := scopeToLimit(s); compareLimits(stat, ls, percentage) {
					result.Services[srv] = stat
				}
			}
		}
		if len(stat.Protocols) > 0 {
			result.Protocols = make(map[string]notOmitEmptyResourceLimit, len(stat.Protocols))
			for proto, s := range stat.Protocols {
				ls := limits.Protocols[string(proto)]
				if stat := scopeToLimit(s); compareLimits(stat, ls, percentage) {
					result.Protocols[string(proto)] = stat
				}
			}
		}
		if len(stat.Peers) > 0 {
			result.Peers = make(map[string]notOmitEmptyResourceLimit, len(stat.Peers))
			for p, s := range stat.Peers {
				ls := limits.Peers[p.Pretty()]
				if stat := scopeToLimit(s); compareLimits(stat, ls, percentage) {
					result.Peers[p.Pretty()] = stat
				}
			}
		}

		return result, nil

	case scope == config.ResourceMgrSystemScope:
		err = mgr.ViewSystem(func(s network.ResourceScope) error {
			stat := scopeToLimit(s.Stat())
			result.System = &stat
			return nil
		})
		return result, err

	case scope == config.ResourceMgrTransientScope:
		err = mgr.ViewTransient(func(s network.ResourceScope) error {
			stat := scopeToLimit(s.Stat())
			result.Transient = &stat
			return nil
		})
		return result, err

	case strings.HasPrefix(scope, config.ResourceMgrServiceScopePrefix):
		svc := strings.TrimPrefix(scope, config.ResourceMgrServiceScopePrefix)
		err = mgr.ViewService(svc, func(s network.ServiceScope) error {
			result.Services = map[string]notOmitEmptyResourceLimit{
				svc: scopeToLimit(s.Stat()),
			}
			return nil
		})
		return result, err

	case strings.HasPrefix(scope, config.ResourceMgrProtocolScopePrefix):
		proto := strings.TrimPrefix(scope, config.ResourceMgrProtocolScopePrefix)
		err = mgr.ViewProtocol(protocol.ID(proto), func(s network.ProtocolScope) error {
			result.Protocols = map[string]notOmitEmptyResourceLimit{
				proto: scopeToLimit(s.Stat()),
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
			result.Peers = map[string]notOmitEmptyResourceLimit{
				p: scopeToLimit(s.Stat()),
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

func scopeToLimit(s network.ScopeStat) notOmitEmptyResourceLimit {
	return notOmitEmptyResourceLimit{
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
// it returns true.
func compareLimits(stat, limit notOmitEmptyResourceLimit, percentage int) bool {
	if abovePercentage(int(stat.Memory), int(limit.Memory), percentage) {
		return true
	}
	if abovePercentage(stat.ConnsInbound, limit.ConnsInbound, percentage) {
		return true
	}
	if abovePercentage(stat.ConnsOutbound, limit.ConnsOutbound, percentage) {
		return true
	}
	if abovePercentage(stat.Conns, limit.Conns, percentage) {
		return true
	}
	if abovePercentage(stat.FD, limit.FD, percentage) {
		return true
	}
	if abovePercentage(stat.StreamsInbound, limit.StreamsInbound, percentage) {
		return true
	}
	if abovePercentage(stat.StreamsOutbound, limit.StreamsOutbound, percentage) {
		return true
	}
	if abovePercentage(stat.Streams, limit.Streams, percentage) {
		return true
	}

	return false
}

func abovePercentage[T constraints.Integer | constraints.Float](v1, v2 T, percentage int) bool {
	if percentage == 0 {
		return true
	}

	if v2 == 0 {
		return false
	}

	return int((float64(v1)/float64(v2))*100) >= percentage
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
			result.System = &s
		case config.ResourceMgrTransientScope:
			s, err := NetLimit(mgr, config.ResourceMgrSystemScope)
			if err != nil {
				return nil, err
			}
			result.Transient = &s
		case config.ResourceMgrServiceScopePrefix:
			result.Services = make(map[string]notOmitEmptyResourceLimit)
			for _, serv := range lister.ListServices() {
				s, err := NetLimit(mgr, config.ResourceMgrServiceScopePrefix+serv)
				if err != nil {
					return nil, err
				}
				result.Services[serv] = s
			}
		case config.ResourceMgrProtocolScopePrefix:
			result.Protocols = make(map[string]notOmitEmptyResourceLimit)
			for _, prot := range lister.ListProtocols() {
				ps := string(prot)
				s, err := NetLimit(mgr, config.ResourceMgrProtocolScopePrefix+ps)
				if err != nil {
					return nil, err
				}
				result.Protocols[ps] = s
			}
		case config.ResourceMgrPeerScopePrefix:
			result.Peers = make(map[string]notOmitEmptyResourceLimit)
			for _, peer := range lister.ListPeers() {
				ps := peer.Pretty()
				s, err := NetLimit(mgr, config.ResourceMgrPeerScopePrefix+ps)
				if err != nil {
					return nil, err
				}
				result.Peers[ps] = s
			}
		}
	}

	return result, nil
}

func NetLimit(mgr network.ResourceManager, scope string) (notOmitEmptyResourceLimit, error) {
	var result rcmgr.ResourceLimits
	getLimit := func(s network.ResourceScope) error {
		limiter, ok := s.(rcmgr.ResourceScopeLimiter)
		if !ok { // NullResourceManager
			return ErrNoResourceMgr
		}

		switch limit := limiter.Limit(); l := limit.(type) {
		case *rcmgr.BaseLimit:
			result = l.ToResourceLimits()
		case rcmgr.BaseLimit:
			result = l.ToResourceLimits()
		default:
			return fmt.Errorf("unknown limit type %T", limit)
		}

		return nil
	}

	var err error
	switch {
	case scope == config.ResourceMgrSystemScope:
		err = mgr.ViewSystem(func(s network.ResourceScope) error { return getLimit(s) })
	case scope == config.ResourceMgrTransientScope:
		err = mgr.ViewTransient(func(s network.ResourceScope) error { return getLimit(s) })
	case strings.HasPrefix(scope, config.ResourceMgrServiceScopePrefix):
		svc := strings.TrimPrefix(scope, config.ResourceMgrServiceScopePrefix)
		err = mgr.ViewService(svc, func(s network.ServiceScope) error { return getLimit(s) })
	case strings.HasPrefix(scope, config.ResourceMgrProtocolScopePrefix):
		proto := strings.TrimPrefix(scope, config.ResourceMgrProtocolScopePrefix)
		err = mgr.ViewProtocol(protocol.ID(proto), func(s network.ProtocolScope) error { return getLimit(s) })
	case strings.HasPrefix(scope, config.ResourceMgrPeerScopePrefix):
		p := strings.TrimPrefix(scope, config.ResourceMgrPeerScopePrefix)
		var pid peer.ID
		pid, err = peer.Decode(p)
		if err != nil {
			return notOmitEmptyResourceLimit{}, fmt.Errorf("invalid peer ID: %q: %w", p, err)
		}
		err = mgr.ViewPeer(pid, func(s network.PeerScope) error { return getLimit(s) })
	default:
		err = fmt.Errorf("invalid scope %q", scope)
	}
	return resourceLimitsToNotOmitEmpty(result), err
}

// NetSetLimit sets new ResourceManager limits for the given scope. The limits take effect immediately, and are also persisted to the repo config.
func NetSetLimit(mgr network.ResourceManager, repo repo.Repo, scope string, limit rcmgr.ResourceLimits) error {
	setLimit := func(s network.ResourceScope) error {
		limiter, ok := s.(rcmgr.ResourceScopeLimiter)
		if !ok { // NullResourceManager
			return ErrNoResourceMgr
		}

		l := rcmgr.InfiniteLimits.ToPartialLimitConfig().System
		limiter.SetLimit(limit.Build(l.Build(rcmgr.BaseLimit{})))
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
			configLimits.Service[svc] = limit
		}
	case strings.HasPrefix(scope, config.ResourceMgrProtocolScopePrefix):
		proto := strings.TrimPrefix(scope, config.ResourceMgrProtocolScopePrefix)
		err = mgr.ViewProtocol(protocol.ID(proto), func(s network.ProtocolScope) error { return setLimit(s) })
		setConfigFunc = func() {
			if configLimits.Protocol == nil {
				configLimits.Protocol = map[protocol.ID]rcmgr.ResourceLimits{}
			}
			configLimits.Protocol[protocol.ID(proto)] = limit
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
			configLimits.Peer[pid] = limit
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
func NetResetLimit(mgr network.ResourceManager, repo repo.Repo, scope string) (rcmgr.BaseLimit, error) {
	var result rcmgr.BaseLimit

	setLimit := func(s network.ResourceScope, l rcmgr.Limit) error {
		limiter, ok := s.(rcmgr.ResourceScopeLimiter)
		if !ok {
			return ErrNoResourceMgr
		}

		limiter.SetLimit(l)
		return nil
	}

	cfg, err := repo.Config()
	if err != nil {
		return rcmgr.BaseLimit{}, fmt.Errorf("reading config to reset limit: %w", err)
	}

	defaultsOrig, err := createDefaultLimitConfig(cfg.Swarm)
	if err != nil {
		return rcmgr.BaseLimit{}, fmt.Errorf("creating default limit config: %w", err)
	}
	defaults := defaultsOrig.ToPartialLimitConfig()

	// INVESTIGATE(@Jorropo): Why do we save scaled configs in the repo ?

	if cfg.Swarm.ResourceMgr.Limits == nil {
		cfg.Swarm.ResourceMgr.Limits = &rcmgr.PartialLimitConfig{}
	}
	configLimits := cfg.Swarm.ResourceMgr.Limits

	var setConfigFunc func() rcmgr.BaseLimit
	switch {
	case scope == config.ResourceMgrSystemScope:
		err = mgr.ViewSystem(func(s network.ResourceScope) error { return setLimit(s, defaults.System.Build(rcmgr.BaseLimit{})) })
		setConfigFunc = func() rcmgr.BaseLimit {
			configLimits.System = defaults.System
			return defaults.System.Build(rcmgr.BaseLimit{})
		}
	case scope == config.ResourceMgrTransientScope:
		err = mgr.ViewTransient(func(s network.ResourceScope) error { return setLimit(s, defaults.Transient.Build(rcmgr.BaseLimit{})) })
		setConfigFunc = func() rcmgr.BaseLimit {
			configLimits.Transient = defaults.Transient
			return defaults.Transient.Build(rcmgr.BaseLimit{})
		}
	case strings.HasPrefix(scope, config.ResourceMgrServiceScopePrefix):
		svc := strings.TrimPrefix(scope, config.ResourceMgrServiceScopePrefix)

		err = mgr.ViewService(svc, func(s network.ServiceScope) error {
			return setLimit(s, defaults.ServiceDefault.Build(rcmgr.BaseLimit{}))
		})
		setConfigFunc = func() rcmgr.BaseLimit {
			if configLimits.Service == nil {
				configLimits.Service = map[string]rcmgr.ResourceLimits{}
			}
			configLimits.Service[svc] = defaults.ServiceDefault
			return defaults.ServiceDefault.Build(rcmgr.BaseLimit{})
		}
	case strings.HasPrefix(scope, config.ResourceMgrProtocolScopePrefix):
		proto := strings.TrimPrefix(scope, config.ResourceMgrProtocolScopePrefix)

		err = mgr.ViewProtocol(protocol.ID(proto), func(s network.ProtocolScope) error {
			return setLimit(s, defaults.ProtocolDefault.Build(rcmgr.BaseLimit{}))
		})
		setConfigFunc = func() rcmgr.BaseLimit {
			if configLimits.Protocol == nil {
				configLimits.Protocol = map[protocol.ID]rcmgr.ResourceLimits{}
			}
			configLimits.Protocol[protocol.ID(proto)] = defaults.ProtocolDefault

			return defaults.ProtocolDefault.Build(rcmgr.BaseLimit{})
		}
	case strings.HasPrefix(scope, config.ResourceMgrPeerScopePrefix):
		p := strings.TrimPrefix(scope, config.ResourceMgrPeerScopePrefix)

		var pid peer.ID
		pid, err = peer.Decode(p)
		if err != nil {
			return result, fmt.Errorf("invalid peer ID: %q: %w", p, err)
		}

		err = mgr.ViewPeer(pid, func(s network.PeerScope) error { return setLimit(s, defaults.PeerDefault.Build(rcmgr.BaseLimit{})) })
		setConfigFunc = func() rcmgr.BaseLimit {
			if configLimits.Peer == nil {
				configLimits.Peer = map[peer.ID]rcmgr.ResourceLimits{}
			}
			configLimits.Peer[pid] = defaults.PeerDefault

			return defaults.PeerDefault.Build(rcmgr.BaseLimit{})
		}
	default:
		return result, fmt.Errorf("invalid scope %q", scope)
	}

	if err != nil {
		return result, fmt.Errorf("resetting new limits on resource manager: %w", err)
	}

	result = setConfigFunc()

	if err := repo.SetConfig(cfg); err != nil {
		return result, fmt.Errorf("writing new limits to repo config: %w", err)
	}

	return result, nil
}

func ensureConnMgrMakeSenseVsResourceMgr(orig rcmgr.ConcreteLimitConfig, cmgr config.ConnMgr) error {
	if cmgr.Type.WithDefault(config.DefaultConnMgrType) == "none" {
		return nil // none connmgr, no checks to do
	}

	rcm := orig.ToPartialLimitConfig()

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
