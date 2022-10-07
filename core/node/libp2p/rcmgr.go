package libp2p

import (
	"context"
	"errors"
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
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	rcmgrObs "github.com/libp2p/go-libp2p/p2p/host/resource-manager/obs"
	"github.com/multiformats/go-multiaddr"
	"go.opencensus.io/stats/view"

	"go.uber.org/fx"
)

const NetLimitDefaultFilename = "limit.json"
const NetLimitTraceFilename = "rcmgr.json.gz"

var ErrNoResourceMgr = fmt.Errorf("missing ResourceMgr: make sure the daemon is running with Swarm.ResourceMgr.Enabled")

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

// TODO ensure that RM is never NullResourceManager inside here, or that it works correctly
func buildScope(scopeStr string, rm network.ResourceManager, repoCfg *config.Config) (rcmgrScope, error) {
	limits := repoCfg.Swarm.ResourceMgr.Limits
	defaults := adjustedDefaultLimits(repoCfg.Swarm)

	// Set maps to non-nil to prevent mistakes downstream.
	// Since these are all "omitempty", they are dropped if nothing is added.
	if limits.Service == nil {
		limits.Service = make(map[string]rcmgr.BaseLimit)
	}
	if limits.Protocol == nil {
		limits.Protocol = make(map[protocol.ID]rcmgr.BaseLimit)
	}
	if limits.Peer == nil {
		limits.Peer = make(map[peer.ID]rcmgr.BaseLimit)
	}
	if limits.ProtocolPeer == nil {
		limits.ProtocolPeer = make(map[protocol.ID]rcmgr.BaseLimit)
	}
	if limits.ServicePeer == nil {
		limits.ServicePeer = make(map[string]rcmgr.BaseLimit)
	}

	switch {
	case scopeStr == "all":
		return nil, nil
	case scopeStr == config.ResourceMgrSystemScope:
		return &rcmgrScopeSystem{rm, limits, defaults}, nil
	case scopeStr == config.ResourceMgrTransientScope:
		return &rcmgrScopeTransient{rm, limits, defaults}, nil
	case strings.HasPrefix(scopeStr, config.ResourceMgrServiceScopePrefix):
		svc := strings.TrimPrefix(scopeStr, config.ResourceMgrServiceScopePrefix)
		return &rcmgrScopeService{svc, rm, limits, defaults}, nil
	default:
		return nil, errors.New("unknown scope")
	}
}

type rcmgrScope interface {
	// Stat returns the the scope's stats from RM.
	Stat() (NetStatOut, error)
	// GetLimit gets the scope's limit from RM.
	GetLimit() (rcmgr.BaseLimit, error)
	// SetLimit sets the given limit on the limits struct and also in RM.
	SetLimit(newLimit rcmgr.BaseLimit) error
	// ResetLimit resets the scope's limits to the defaults on the limits struct and also in RM.
	ResetLimits() (rcmgr.BaseLimit, error)
}

func getRMLimit(s network.ResourceScope) (rcmgr.BaseLimit, error) {
	var result rcmgr.BaseLimit
	limiter, ok := s.(rcmgr.ResourceScopeLimiter)
	if !ok { // NullResourceManager
		return result, ErrNoResourceMgr
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
		return result, fmt.Errorf("unknown limit type %T", limit)
	}

	return result, nil
}

func setRMLimit(s network.ResourceScope, limit rcmgr.BaseLimit) error {
	limiter, ok := s.(rcmgr.ResourceScopeLimiter)
	if !ok { // NullResourceManager
		return fmt.Errorf("setting new limits on resource manager: %w", ErrNoResourceMgr)
	}
	limiter.SetLimit(&limit)
	return nil
}

type rcmgrScopeAll struct {
	mgr rcmgr.ResourceManagerState
}

func (s *rcmgrScopeAll) Stat() (NetStatOut, error) {
	var result NetStatOut
	stat := s.mgr.Stat()
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
}

func (s *rcmgrScopeAll) GetLimit() (rcmgr.BaseLimit, error) {
	return rcmgr.BaseLimit{}, errors.New(`invalid scope "all"`)
}

func (s *rcmgrScopeAll) SetLimit(_ *rcmgr.LimitConfig, _ rcmgr.BaseLimit) error {
	return errors.New(`invalid scope "all"`)
}

func (s *rcmgrScopeAll) ResetLimits(_ *rcmgr.LimitConfig) error {
	return errors.New(`invalid scope "all"`)
}

type rcmgrScopeSystem struct {
	mgr      network.ResourceManager
	limits   *rcmgr.LimitConfig
	defaults rcmgr.LimitConfig
}

func (s *rcmgrScopeSystem) Stat() (NetStatOut, error) {
	var result NetStatOut
	err := s.mgr.ViewSystem(func(rs network.ResourceScope) error {
		stat := rs.Stat()
		result.System = &stat
		return nil
	})
	return result, err
}

func (s *rcmgrScopeSystem) GetLimit() (result rcmgr.BaseLimit, err error) {
	err = s.mgr.ViewSystem(func(rs network.ResourceScope) error {
		limit, err := getRMLimit(rs)
		result = limit
		return err
	})
	return
}

func (s *rcmgrScopeSystem) SetLimit(limit rcmgr.BaseLimit) error {
	err := s.mgr.ViewSystem(func(rs network.ResourceScope) error {
		return setRMLimit(rs, limit)
	})
	if err != nil {
		return err
	}
	s.limits.System = limit
	return nil
}

func (s *rcmgrScopeSystem) ResetLimits() (result rcmgr.BaseLimit, err error) {
	err = s.mgr.ViewSystem(func(rs network.ResourceScope) error {
		err := setRMLimit(rs, s.defaults.System)
		result = s.defaults.System
		return err
	})
	if err != nil {
		return
	}
	s.limits.System = s.defaults.System
	return
}

type rcmgrScopeTransient struct {
	mgr      network.ResourceManager
	limits   *rcmgr.LimitConfig
	defaults rcmgr.LimitConfig
}

func (s *rcmgrScopeTransient) Stat() (NetStatOut, error) {
	var result NetStatOut
	err := s.mgr.ViewTransient(func(rs network.ResourceScope) error {
		stat := rs.Stat()
		result.System = &stat
		return nil
	})
	return result, err
}

func (s *rcmgrScopeTransient) GetLimit() (result rcmgr.BaseLimit, err error) {
	err = s.mgr.ViewTransient(func(rs network.ResourceScope) error {
		limit, err := getRMLimit(rs)
		result = limit
		return err
	})
	return
}

func (s *rcmgrScopeTransient) SetLimit(limit rcmgr.BaseLimit) error {
	err := s.mgr.ViewTransient(func(rs network.ResourceScope) error {
		return setRMLimit(rs, limit)
	})
	if err != nil {
		return err
	}
	s.limits.Transient = limit
	return nil
}

func (s *rcmgrScopeTransient) ResetLimits() (result rcmgr.BaseLimit, err error) {
	err = s.mgr.ViewTransient(func(rs network.ResourceScope) error {
		err := setRMLimit(rs, s.defaults.Transient)
		result = s.defaults.Transient
		return err
	})
	if err != nil {
		return
	}
	s.limits.Transient = s.defaults.Transient
	return
}

type rcmgrScopeService struct {
	service  string
	mgr      network.ResourceManager
	limits   *rcmgr.LimitConfig
	defaults rcmgr.LimitConfig
}

func (s *rcmgrScopeService) Stat() (NetStatOut, error) {
	var result NetStatOut
	err := s.mgr.ViewService(s.service, func(ss network.ServiceScope) error {
		result.Services = map[string]network.ScopeStat{s.service: ss.Stat()}
		return nil
	})
	return result, err
}

func (s *rcmgrScopeService) GetLimit() (result rcmgr.BaseLimit, err error) {
	err = s.mgr.ViewService(s.service, func(ss network.ServiceScope) error {
		limit, err := getRMLimit(ss)
		result = limit
		return err
	})
	return
}

func (s *rcmgrScopeService) SetLimit(limit rcmgr.BaseLimit) error {
	err := s.mgr.ViewService(s.service, func(ss network.ServiceScope) error {
		return setRMLimit(ss, limit)
	})
	if err != nil {
		return err
	}
	s.limits.Service[s.service] = limit
	return nil
}

func (s *rcmgrScopeService) ResetLimits() (result rcmgr.BaseLimit, err error) {
	err = s.mgr.ViewService(s.service, func(ss network.ServiceScope) error {
		err := setRMLimit(ss, s.defaults.ServiceDefault)
		result = s.defaults.ServiceDefault
		return err
	})
	if err != nil {
		return
	}
	s.limits.Service[s.service] = s.defaults.ServiceDefault
	return
}

type NetStatOut struct {
	System    *network.ScopeStat           `json:",omitempty"`
	Transient *network.ScopeStat           `json:",omitempty"`
	Services  map[string]network.ScopeStat `json:",omitempty"`
	Protocols map[string]network.ScopeStat `json:",omitempty"`
	Peers     map[string]network.ScopeStat `json:",omitempty"`
}

func NetStat(scopeStr string, rm network.ResourceManager, repo repo.Repo) (NetStatOut, error) {
	var result NetStatOut

	cfg, err := repo.Config()
	if err != nil {
		return result, fmt.Errorf("reading repo config: %w", err)
	}

	scope, err := buildScope(scopeStr, rm, cfg)
	if err != nil {
		return result, err
	}
	return scope.Stat()
}

// func NetStat(mgr network.ResourceManager, scope string) (NetStatOut, error) {
// 	var err error
// 	var result NetStatOut
// 	switch {
// 	case scope == "all":
// 		rapi, ok := mgr.(rcmgr.ResourceManagerState)
// 		if !ok { // NullResourceManager
// 			return result, ErrNoResourceMgr
// 		}

// 		stat := rapi.Stat()
// 		result.System = &stat.System
// 		result.Transient = &stat.Transient
// 		if len(stat.Services) > 0 {
// 			result.Services = stat.Services
// 		}
// 		if len(stat.Protocols) > 0 {
// 			result.Protocols = make(map[string]network.ScopeStat, len(stat.Protocols))
// 			for proto, stat := range stat.Protocols {
// 				result.Protocols[string(proto)] = stat
// 			}
// 		}
// 		if len(stat.Peers) > 0 {
// 			result.Peers = make(map[string]network.ScopeStat, len(stat.Peers))
// 			for p, stat := range stat.Peers {
// 				result.Peers[p.Pretty()] = stat
// 			}
// 		}

// 		return result, nil

// 	case scope == config.ResourceMgrSystemScope:
// 		err = mgr.ViewSystem(func(s network.ResourceScope) error {
// 			stat := s.Stat()
// 			result.System = &stat
// 			return nil
// 		})
// 		return result, err

// 	case scope == config.ResourceMgrTransientScope:
// 		err = mgr.ViewTransient(func(s network.ResourceScope) error {
// 			stat := s.Stat()
// 			result.Transient = &stat
// 			return nil
// 		})
// 		return result, err

// 	case strings.HasPrefix(scope, config.ResourceMgrServiceScopePrefix):
// 		svc := strings.TrimPrefix(scope, config.ResourceMgrServiceScopePrefix)
// 		err = mgr.ViewService(svc, func(s network.ServiceScope) error {
// 			stat := s.Stat()
// 			result.Services = map[string]network.ScopeStat{
// 				svc: stat,
// 			}
// 			return nil
// 		})
// 		return result, err

// 	case strings.HasPrefix(scope, config.ResourceMgrProtocolScopePrefix):
// 		proto := strings.TrimPrefix(scope, config.ResourceMgrProtocolScopePrefix)
// 		err = mgr.ViewProtocol(protocol.ID(proto), func(s network.ProtocolScope) error {
// 			stat := s.Stat()
// 			result.Protocols = map[string]network.ScopeStat{
// 				proto: stat,
// 			}
// 			return nil
// 		})
// 		return result, err

// 	case strings.HasPrefix(scope, config.ResourceMgrPeerScopePrefix):
// 		p := strings.TrimPrefix(scope, config.ResourceMgrPeerScopePrefix)
// 		pid, err := peer.Decode(p)
// 		if err != nil {
// 			return result, fmt.Errorf("invalid peer ID: %q: %w", p, err)
// 		}
// 		err = mgr.ViewPeer(pid, func(s network.PeerScope) error {
// 			stat := s.Stat()
// 			result.Peers = map[string]network.ScopeStat{
// 				p: stat,
// 			}
// 			return nil
// 		})
// 		return result, err

// 	default:
// 		return result, fmt.Errorf("invalid scope %q", scope)
// 	}
// }

func NetLimit(scopeStr string, rm network.ResourceManager, repo repo.Repo) (rcmgr.BaseLimit, error) {
	var result rcmgr.BaseLimit

	cfg, err := repo.Config()
	if err != nil {
		return result, fmt.Errorf("reading repo config: %w", err)
	}

	scope, err := buildScope(scopeStr, rm, cfg)
	if err != nil {
		return result, err
	}
	return scope.GetLimit()
}

// func NetLimit(mgr network.ResourceManager, scope string) (rcmgr.BaseLimit, error) {
// 	var result rcmgr.BaseLimit
// 	getLimit := func(s network.ResourceScope) error {
// 		limiter, ok := s.(rcmgr.ResourceScopeLimiter)
// 		if !ok { // NullResourceManager
// 			return ErrNoResourceMgr
// 		}
// 		limit := limiter.Limit()
// 		switch l := limit.(type) {
// 		case *rcmgr.BaseLimit:
// 			result.Memory = l.Memory
// 			result.Streams = l.Streams
// 			result.StreamsInbound = l.StreamsInbound
// 			result.StreamsOutbound = l.StreamsOutbound
// 			result.Conns = l.Conns
// 			result.ConnsInbound = l.ConnsInbound
// 			result.ConnsOutbound = l.ConnsOutbound
// 			result.FD = l.FD
// 		default:
// 			return fmt.Errorf("unknown limit type %T", limit)
// 		}

// 		return nil
// 	}

// 	switch {
// 	case scope == config.ResourceMgrSystemScope:
// 		return result, mgr.ViewSystem(func(s network.ResourceScope) error { return getLimit(s) })
// 	case scope == config.ResourceMgrTransientScope:
// 		return result, mgr.ViewTransient(func(s network.ResourceScope) error { return getLimit(s) })
// 	case strings.HasPrefix(scope, config.ResourceMgrServiceScopePrefix):
// 		svc := strings.TrimPrefix(scope, config.ResourceMgrServiceScopePrefix)
// 		return result, mgr.ViewService(svc, func(s network.ServiceScope) error { return getLimit(s) })
// 	case strings.HasPrefix(scope, config.ResourceMgrProtocolScopePrefix):
// 		proto := strings.TrimPrefix(scope, config.ResourceMgrProtocolScopePrefix)
// 		return result, mgr.ViewProtocol(protocol.ID(proto), func(s network.ProtocolScope) error { return getLimit(s) })
// 	case strings.HasPrefix(scope, config.ResourceMgrPeerScopePrefix):
// 		p := strings.TrimPrefix(scope, config.ResourceMgrPeerScopePrefix)
// 		pid, err := peer.Decode(p)
// 		if err != nil {
// 			return result, fmt.Errorf("invalid peer ID: %q: %w", p, err)
// 		}
// 		return result, mgr.ViewPeer(pid, func(s network.PeerScope) error { return getLimit(s) })
// 	default:
// 		return result, fmt.Errorf("invalid scope %q", scope)
// 	}
// }

// NetSetLimit sets new ResourceManager limits for the given scope. The limits take effect immediately, and are also persisted to the repo config.
func NetSetLimit(rm network.ResourceManager, repo repo.Repo, scopeStr string, limit rcmgr.BaseLimit) error {
	cfg, err := repo.Config()
	if err != nil {
		return fmt.Errorf("reading repo config: %w", err)
	}

	scope, err := buildScope(scopeStr, rm, cfg)
	if err != nil {
		return err
	}
	err = scope.SetLimit(limit)
	if err != nil {
		return err
	}

	return repo.SetConfig(cfg)
}

// func NetSetLimit(mgr network.ResourceManager, repo repo.Repo, scope string, limit rcmgr.BaseLimit) error {
// 	setLimit := func(s network.ResourceScope) error {
// 		limiter, ok := s.(rcmgr.ResourceScopeLimiter)
// 		if !ok { // NullResourceManager
// 			return ErrNoResourceMgr
// 		}

// 		limiter.SetLimit(&limit)
// 		return nil
// 	}

// 	cfg, err := repo.Config()
// 	if err != nil {
// 		return fmt.Errorf("reading config to set limit: %w", err)
// 	}

// 	if cfg.Swarm.ResourceMgr.Limits == nil {
// 		cfg.Swarm.ResourceMgr.Limits = &rcmgr.LimitConfig{}
// 	}
// 	configLimits := cfg.Swarm.ResourceMgr.Limits

// 	var setConfigFunc func()
// 	switch {
// 	case scope == config.ResourceMgrSystemScope:
// 		err = mgr.ViewSystem(func(s network.ResourceScope) error { return setLimit(s) })
// 		setConfigFunc = func() { configLimits.System = limit }
// 	case scope == config.ResourceMgrTransientScope:
// 		err = mgr.ViewTransient(func(s network.ResourceScope) error { return setLimit(s) })
// 		setConfigFunc = func() { configLimits.Transient = limit }
// 	case strings.HasPrefix(scope, config.ResourceMgrServiceScopePrefix):
// 		svc := strings.TrimPrefix(scope, config.ResourceMgrServiceScopePrefix)
// 		err = mgr.ViewService(svc, func(s network.ServiceScope) error { return setLimit(s) })
// 		setConfigFunc = func() {
// 			if configLimits.Service == nil {
// 				configLimits.Service = map[string]rcmgr.BaseLimit{}
// 			}
// 			configLimits.Service[svc] = limit
// 		}
// 	case strings.HasPrefix(scope, config.ResourceMgrProtocolScopePrefix):
// 		proto := strings.TrimPrefix(scope, config.ResourceMgrProtocolScopePrefix)
// 		err = mgr.ViewProtocol(protocol.ID(proto), func(s network.ProtocolScope) error { return setLimit(s) })
// 		setConfigFunc = func() {
// 			if configLimits.Protocol == nil {
// 				configLimits.Protocol = map[protocol.ID]rcmgr.BaseLimit{}
// 			}
// 			configLimits.Protocol[protocol.ID(proto)] = limit
// 		}
// 	case strings.HasPrefix(scope, config.ResourceMgrPeerScopePrefix):
// 		p := strings.TrimPrefix(scope, config.ResourceMgrPeerScopePrefix)
// 		var pid peer.ID
// 		pid, err = peer.Decode(p)
// 		if err != nil {
// 			return fmt.Errorf("invalid peer ID: %q: %w", p, err)
// 		}
// 		err = mgr.ViewPeer(pid, func(s network.PeerScope) error { return setLimit(s) })
// 		setConfigFunc = func() {
// 			if configLimits.Peer == nil {
// 				configLimits.Peer = map[peer.ID]rcmgr.BaseLimit{}
// 			}
// 			configLimits.Peer[pid] = limit
// 		}
// 	default:
// 		return fmt.Errorf("invalid scope %q", scope)
// 	}

// 	if err != nil {
// 		return fmt.Errorf("setting new limits on resource manager: %w", err)
// 	}

// 	if cfg.Swarm.ResourceMgr.Limits == nil {
// 		cfg.Swarm.ResourceMgr.Limits = &rcmgr.LimitConfig{}
// 	}
// 	setConfigFunc()

// 	if err := repo.SetConfig(cfg); err != nil {
// 		return fmt.Errorf("writing new limits to repo config: %w", err)
// 	}

// 	return nil
// }

// NetResetLimit resets ResourceManager limits to defaults. The limits take effect immediately, and are also persisted to the repo config.
func NetResetLimit(rm network.ResourceManager, repo repo.Repo, scopeStr string) (rcmgr.BaseLimit, error) {
	var result rcmgr.BaseLimit

	cfg, err := repo.Config()
	if err != nil {
		return result, fmt.Errorf("reading repo config: %w", err)
	}

	scope, err := buildScope(scopeStr, rm, cfg)
	if err != nil {
		return result, err
	}
	result, err = scope.ResetLimits()
	if err != nil {
		return result, err
	}

	err = repo.SetConfig(cfg)

	return result, err
}

// // NetResetLimit resets ResourceManager limits to defaults. The limits take effect immediately, and are also persisted to the repo config.
// func NetResetLimit(mgr network.ResourceManager, repo repo.Repo, scope string) (rcmgr.BaseLimit, error) {
// 	var result rcmgr.BaseLimit

// 	setLimit := func(s network.ResourceScope, l rcmgr.Limit) error {
// 		limiter, ok := s.(rcmgr.ResourceScopeLimiter)
// 		if !ok {
// 			return ErrNoResourceMgr
// 		}

// 		limiter.SetLimit(l)
// 		return nil
// 	}

// 	cfg, err := repo.Config()
// 	if err != nil {
// 		return result, fmt.Errorf("reading config to reset limit: %w", err)
// 	}

// 	defaults := adjustedDefaultLimits(cfg.Swarm)

// 	if cfg.Swarm.ResourceMgr.Limits == nil {
// 		cfg.Swarm.ResourceMgr.Limits = &rcmgr.LimitConfig{}
// 	}
// 	configLimits := cfg.Swarm.ResourceMgr.Limits

// 	var setConfigFunc func() rcmgr.BaseLimit
// 	switch {
// 	case scope == config.ResourceMgrSystemScope:
// 		err = mgr.ViewSystem(func(s network.ResourceScope) error { return setLimit(s, &defaults.System) })
// 		setConfigFunc = func() rcmgr.BaseLimit {
// 			configLimits.System = defaults.System
// 			return defaults.System
// 		}
// 	case scope == config.ResourceMgrTransientScope:
// 		err = mgr.ViewTransient(func(s network.ResourceScope) error { return setLimit(s, &defaults.Transient) })
// 		setConfigFunc = func() rcmgr.BaseLimit {
// 			configLimits.Transient = defaults.Transient
// 			return defaults.Transient
// 		}
// 	case strings.HasPrefix(scope, config.ResourceMgrServiceScopePrefix):
// 		svc := strings.TrimPrefix(scope, config.ResourceMgrServiceScopePrefix)

// 		if svc == "" {
// 			// reset defaults
// 			err = mgr.ViewSystem(func(s network.ResourceScope) error { return setLimit(s, &defaults.ServiceDefault) })
// 			setConfigFunc = func() rcmgr.BaseLimit {
// 				configLimits.ServiceDefault = defaults.ServiceDefault
// 				return defaults.ServiceDefault
// 			}
// 			break
// 		}

// 		err = mgr.ViewService(svc, func(s network.ServiceScope) error { return setLimit(s, &defaults.ServiceDefault) })
// 		setConfigFunc = func() rcmgr.BaseLimit {
// 			if configLimits.Service == nil {
// 				configLimits.Service = map[string]rcmgr.BaseLimit{}
// 			}
// 			configLimits.Service[svc] = defaults.ServiceDefault
// 			return defaults.ServiceDefault
// 		}
// 	case strings.HasPrefix(scope, config.ResourceMgrProtocolScopePrefix):
// 		proto := strings.TrimPrefix(scope, config.ResourceMgrProtocolScopePrefix)

// 		if proto == "" {
// 			// reset defaults
// 			err = mgr.ViewSystem(func(s network.ResourceScope) error { return setLimit(s, &defaults.ProtocolDefault) })
// 			setConfigFunc = func() rcmgr.BaseLimit {
// 				configLimits.ProtocolDefault = defaults.ProtocolDefault

// 				return defaults.ProtocolDefault
// 			}
// 			break
// 		}

// 		err = mgr.ViewProtocol(protocol.ID(proto), func(s network.ProtocolScope) error { return setLimit(s, &defaults.ProtocolDefault) })
// 		setConfigFunc = func() rcmgr.BaseLimit {
// 			if configLimits.Protocol == nil {
// 				configLimits.Protocol = map[protocol.ID]rcmgr.BaseLimit{}
// 			}
// 			configLimits.Protocol[protocol.ID(proto)] = defaults.ProtocolDefault

// 			return defaults.ProtocolDefault
// 		}
// 	case strings.HasPrefix(scope, config.ResourceMgrPeerScopePrefix):
// 		p := strings.TrimPrefix(scope, config.ResourceMgrPeerScopePrefix)

// 		if p == "" {
// 			// reset defaults
// 			err = mgr.ViewSystem(func(s network.ResourceScope) error { return setLimit(s, &defaults.PeerDefault) })
// 			setConfigFunc = func() rcmgr.BaseLimit {
// 				configLimits.PeerDefault = defaults.PeerDefault

// 				return defaults.PeerDefault
// 			}
// 			break
// 		}

// 		var pid peer.ID
// 		pid, err = peer.Decode(p)
// 		if err != nil {
// 			return result, fmt.Errorf("invalid peer ID: %q: %w", p, err)
// 		}

// 		err = mgr.ViewPeer(pid, func(s network.PeerScope) error { return setLimit(s, &defaults.PeerDefault) })
// 		setConfigFunc = func() rcmgr.BaseLimit {
// 			if configLimits.Peer == nil {
// 				configLimits.Peer = map[peer.ID]rcmgr.BaseLimit{}
// 			}
// 			configLimits.Peer[pid] = defaults.PeerDefault

// 			return defaults.PeerDefault
// 		}
// 	default:
// 		return result, fmt.Errorf("invalid scope %q", scope)
// 	}

// 	if err != nil {
// 		return result, fmt.Errorf("resetting new limits on resource manager: %w", err)
// 	}

// 	result = setConfigFunc()

// 	if err := repo.SetConfig(cfg); err != nil {
// 		return result, fmt.Errorf("writing new limits to repo config: %w", err)
// 	}

// 	return result, nil
// }
