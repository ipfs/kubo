package libp2p

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/benbjohnson/clock"
	logging "github.com/ipfs/go-log/v2"
	config "github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/node/helpers"
	"github.com/ipfs/kubo/repo"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	rcmgr "github.com/libp2p/go-libp2p-resource-manager"
	rcmgrObs "github.com/libp2p/go-libp2p-resource-manager/obs"
	"github.com/multiformats/go-multiaddr"
	"go.opencensus.io/stats/view"

	"go.uber.org/fx"
)

const NetLimitDefaultFilename = "limit.json"
const NetLimitTraceFilename = "rcmgr.json.gz"

var NoResourceMgrError = fmt.Errorf("missing ResourceMgr: make sure the daemon is running with Swarm.ResourceMgr.Enabled")

func ResourceManager(cfg config.SwarmConfig) interface{} {
	return func(mctx helpers.MetricsCtx, lc fx.Lifecycle, repo repo.Repo) (network.ResourceManager, Libp2pOpts, error) {
		var manager network.ResourceManager
		var opts Libp2pOpts

		enabled := cfg.ResourceMgr.Enabled.WithDefault(false)

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

			limits := adjustedDefaultLimits(cfg)

			if cfg.ResourceMgr.Limits != nil {
				l := *cfg.ResourceMgr.Limits
				l.Apply(limits)
				limits = l
			}

			limiter := rcmgr.NewFixedLimiter(limits)

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
			log.Debug("libp2p resource manager is disabled")
			manager = network.NullResourceManager
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
	System    *network.ScopeStat           `json:",omitempty"`
	Transient *network.ScopeStat           `json:",omitempty"`
	Services  map[string]network.ScopeStat `json:",omitempty"`
	Protocols map[string]network.ScopeStat `json:",omitempty"`
	Peers     map[string]network.ScopeStat `json:",omitempty"`
}

func NetStat(mgr network.ResourceManager, scope string) (NetStatOut, error) {
	var err error
	var result NetStatOut
	switch {
	case scope == "all":
		rapi, ok := mgr.(rcmgr.ResourceManagerState)
		if !ok { // NullResourceManager
			return result, NoResourceMgrError
		}

		stat := rapi.Stat()
		result.System = &stat.System
		result.Transient = &stat.Transient
		if len(stat.Services) > 0 {
			result.Services = stat.Services
		}
		if len(stat.Protocols) > 0 {
			result.Protocols = make(map[string]network.ScopeStat, len(stat.Protocols))
			for proto, stat := range stat.Protocols {
				result.Protocols[string(proto)] = stat
			}
		}
		if len(stat.Peers) > 0 {
			result.Peers = make(map[string]network.ScopeStat, len(stat.Peers))
			for p, stat := range stat.Peers {
				result.Peers[p.Pretty()] = stat
			}
		}

		return result, nil

	case scope == config.ResourceMgrSystemScope:
		err = mgr.ViewSystem(func(s network.ResourceScope) error {
			stat := s.Stat()
			result.System = &stat
			return nil
		})
		return result, err

	case scope == config.ResourceMgrTransientScope:
		err = mgr.ViewTransient(func(s network.ResourceScope) error {
			stat := s.Stat()
			result.Transient = &stat
			return nil
		})
		return result, err

	case strings.HasPrefix(scope, config.ResourceMgrServiceScopePrefix):
		svc := strings.TrimPrefix(scope, config.ResourceMgrServiceScopePrefix)
		err = mgr.ViewService(svc, func(s network.ServiceScope) error {
			stat := s.Stat()
			result.Services = map[string]network.ScopeStat{
				svc: stat,
			}
			return nil
		})
		return result, err

	case strings.HasPrefix(scope, config.ResourceMgrProtocolScopePrefix):
		proto := strings.TrimPrefix(scope, config.ResourceMgrProtocolScopePrefix)
		err = mgr.ViewProtocol(protocol.ID(proto), func(s network.ProtocolScope) error {
			stat := s.Stat()
			result.Protocols = map[string]network.ScopeStat{
				proto: stat,
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
			result.Peers = map[string]network.ScopeStat{
				p: stat,
			}
			return nil
		})
		return result, err

	default:
		return result, fmt.Errorf("invalid scope %q", scope)
	}
}

func NetLimit(mgr network.ResourceManager, scope string) (rcmgr.BaseLimit, error) {
	var result rcmgr.BaseLimit
	getLimit := func(s network.ResourceScope) error {
		limiter, ok := s.(rcmgr.ResourceScopeLimiter)
		if !ok { // NullResourceManager
			return NoResourceMgrError
		}
		limit := limiter.Limit()
		switch l := limit.(type) {
		case *rcmgr.BaseLimit:
			result.Memory = l.Memory
			result.Streams = l.Streams
			result.StreamsInbound = l.StreamsInbound
			result.StreamsOutbound = l.StreamsOutbound
			result.Conns = l.Conns
			result.ConnsInbound = l.ConnsInbound
			result.ConnsOutbound = l.ConnsOutbound
			result.FD = l.FD
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
func NetSetLimit(mgr network.ResourceManager, repo repo.Repo, scope string, limit rcmgr.BaseLimit) error {
	setLimit := func(s network.ResourceScope) error {
		limiter, ok := s.(rcmgr.ResourceScopeLimiter)
		if !ok { // NullResourceManager
			return NoResourceMgrError
		}

		limiter.SetLimit(&limit)
		return nil
	}

	cfg, err := repo.Config()
	if err != nil {
		return fmt.Errorf("reading config to set limit: %w", err)
	}

	if cfg.Swarm.ResourceMgr.Limits == nil {
		cfg.Swarm.ResourceMgr.Limits = &rcmgr.LimitConfig{}
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
				configLimits.Service = map[string]rcmgr.BaseLimit{}
			}
			configLimits.Service[svc] = limit
		}
	case strings.HasPrefix(scope, config.ResourceMgrProtocolScopePrefix):
		proto := strings.TrimPrefix(scope, config.ResourceMgrProtocolScopePrefix)
		err = mgr.ViewProtocol(protocol.ID(proto), func(s network.ProtocolScope) error { return setLimit(s) })
		setConfigFunc = func() {
			if configLimits.Protocol == nil {
				configLimits.Protocol = map[protocol.ID]rcmgr.BaseLimit{}
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
				configLimits.Peer = map[peer.ID]rcmgr.BaseLimit{}
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
		cfg.Swarm.ResourceMgr.Limits = &rcmgr.LimitConfig{}
	}
	setConfigFunc()

	if err := repo.SetConfig(cfg); err != nil {
		return fmt.Errorf("writing new limits to repo config: %w", err)
	}

	return nil
}
