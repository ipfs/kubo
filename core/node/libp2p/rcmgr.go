package libp2p

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	config "github.com/ipfs/go-ipfs/config"
	"github.com/ipfs/go-ipfs/repo"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	rcmgr "github.com/libp2p/go-libp2p-resource-manager"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/fx"
)

const NetLimitDefaultFilename = "limit.json"
const NetLimitTraceFilename = "rcmgr.json.gz"

var NoResourceMgrError = fmt.Errorf("missing ResourceMgr: make sure the daemon is running with Swarm.ResourceMgr.Enabled")

func ResourceManager(cfg config.ResourceMgr) func(fx.Lifecycle, repo.Repo) (network.ResourceManager, Libp2pOpts, error) {
	return func(lc fx.Lifecycle, repo repo.Repo) (network.ResourceManager, Libp2pOpts, error) {
		var limiter *rcmgr.BasicLimiter
		var manager network.ResourceManager
		var opts Libp2pOpts

		// Config Swarm.ResourceMgr.Enabled decides if we run a real manager
		enabled := cfg.Enabled.WithDefault(false)

		/// ENV overrides Config (if present)
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
				return nil, opts, fmt.Errorf("error opening IPFS_PATH: %w", err)
			}

			// Try defaults from limit.json if provided
			// (a convention to make libp2p team life easier)
			limitFilePath := filepath.Join(repoPath, NetLimitDefaultFilename)
			_, err = os.Stat(limitFilePath)
			if !errors.Is(err, os.ErrNotExist) {
				limitFile, err := os.Open(limitFilePath)
				if err != nil {
					return nil, opts, fmt.Errorf("error opening limit JSON file %q: %w", limitFilePath, err)
				}
				defer limitFile.Close() //nolint:errcheck
				limiter, err = rcmgr.NewDefaultLimiterFromJSON(limitFile)
				if err != nil {
					return nil, opts, fmt.Errorf("error parsing limit file: %w", err)
				}

			} else {
				// Use defaults from go-libp2p
				log.Debug("limit file %s not found, creating a default resource manager", NetLimitDefaultFilename)
				limiter = rcmgr.NewDefaultLimiter()
			}

			libp2p.SetDefaultServiceLimits(limiter)

			ropts := []rcmgr.Option{rcmgr.WithMetrics(&rcmgrMetrics{})}
			if os.Getenv("LIBP2P_DEBUG_RCMGR") != "" {
				traceFilePath := filepath.Join(repoPath, NetLimitTraceFilename)
				ropts = append(ropts, rcmgr.WithTrace(traceFilePath))
			}

			manager, err = rcmgr.NewResourceManager(limiter, ropts...)
			if err != nil {
				return nil, opts, fmt.Errorf("error creating resource manager: %w", err)
			}

			// Apply user-defined Swarm.ResourceMgr.Limits
			for scope, userLimit := range cfg.Limits {
				err := NetSetLimit(manager, scope, userLimit)
				if err != nil {
					return nil, opts, fmt.Errorf("error while applying Swarm.ResourceMgr.Limits for scope %q: %w", scope, err)
				}
			}

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
		svc := scope[4:]
		err = mgr.ViewService(svc, func(s network.ServiceScope) error {
			stat := s.Stat()
			result.Services = map[string]network.ScopeStat{
				svc: stat,
			}
			return nil
		})
		return result, err

	case strings.HasPrefix(scope, config.ResourceMgrProtocolScopePrefix):
		proto := scope[6:]
		err = mgr.ViewProtocol(protocol.ID(proto), func(s network.ProtocolScope) error {
			stat := s.Stat()
			result.Protocols = map[string]network.ScopeStat{
				proto: stat,
			}
			return nil
		})
		return result, err

	case strings.HasPrefix(scope, config.ResourceMgrPeerScopePrefix):
		p := scope[5:]
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

func NetLimit(mgr network.ResourceManager, scope string) (config.ResourceMgrScopeConfig, error) {
	var result config.ResourceMgrScopeConfig
	getLimit := func(s network.ResourceScope) error {
		limiter, ok := s.(rcmgr.ResourceScopeLimiter)
		if !ok { // NullResourceManager
			return NoResourceMgrError
		}

		limit := limiter.Limit()
		switch l := limit.(type) {
		case *rcmgr.StaticLimit:
			result.Dynamic = false
			result.Memory = l.Memory
			result.Streams = l.BaseLimit.Streams
			result.StreamsInbound = l.BaseLimit.StreamsInbound
			result.StreamsOutbound = l.BaseLimit.StreamsOutbound
			result.Conns = l.BaseLimit.Conns
			result.ConnsInbound = l.BaseLimit.ConnsInbound
			result.ConnsOutbound = l.BaseLimit.ConnsOutbound
			result.FD = l.BaseLimit.FD

		case *rcmgr.DynamicLimit:
			result.Dynamic = true
			result.MemoryFraction = l.MemoryLimit.MemoryFraction
			result.MinMemory = l.MemoryLimit.MinMemory
			result.MaxMemory = l.MemoryLimit.MaxMemory
			result.Streams = l.BaseLimit.Streams
			result.StreamsInbound = l.BaseLimit.StreamsInbound
			result.StreamsOutbound = l.BaseLimit.StreamsOutbound
			result.Conns = l.BaseLimit.Conns
			result.ConnsInbound = l.BaseLimit.ConnsInbound
			result.ConnsOutbound = l.BaseLimit.ConnsOutbound
			result.FD = l.BaseLimit.FD

		default:
			return fmt.Errorf("unknown limit type %T", limit)
		}

		return nil
	}

	switch {
	case scope == config.ResourceMgrSystemScope:
		err := mgr.ViewSystem(func(s network.ResourceScope) error {
			return getLimit(s)
		})
		return result, err

	case scope == config.ResourceMgrTransientScope:
		err := mgr.ViewTransient(func(s network.ResourceScope) error {
			return getLimit(s)
		})
		return result, err

	case strings.HasPrefix(scope, config.ResourceMgrServiceScopePrefix):
		svc := scope[4:]
		err := mgr.ViewService(svc, func(s network.ServiceScope) error {
			return getLimit(s)
		})
		return result, err

	case strings.HasPrefix(scope, config.ResourceMgrProtocolScopePrefix):
		proto := scope[6:]
		err := mgr.ViewProtocol(protocol.ID(proto), func(s network.ProtocolScope) error {
			return getLimit(s)
		})
		return result, err

	case strings.HasPrefix(scope, config.ResourceMgrPeerScopePrefix):
		p := scope[5:]
		pid, err := peer.Decode(p)
		if err != nil {
			return result, fmt.Errorf("invalid peer ID: %q: %w", p, err)
		}
		err = mgr.ViewPeer(pid, func(s network.PeerScope) error {
			return getLimit(s)
		})
		return result, err

	default:
		return result, fmt.Errorf("invalid scope %q", scope)
	}
}

func NetSetLimit(mgr network.ResourceManager, scope string, limit config.ResourceMgrScopeConfig) error {
	setLimit := func(s network.ResourceScope) error {
		limiter, ok := s.(rcmgr.ResourceScopeLimiter)
		if !ok { // NullResourceManager
			return NoResourceMgrError
		}

		var newLimit rcmgr.Limit
		if limit.Dynamic {
			newLimit = &rcmgr.DynamicLimit{
				MemoryLimit: rcmgr.MemoryLimit{
					MemoryFraction: limit.MemoryFraction,
					MinMemory:      limit.MinMemory,
					MaxMemory:      limit.MaxMemory,
				},
				BaseLimit: rcmgr.BaseLimit{
					Streams:         limit.Streams,
					StreamsInbound:  limit.StreamsInbound,
					StreamsOutbound: limit.StreamsOutbound,
					Conns:           limit.Conns,
					ConnsInbound:    limit.ConnsInbound,
					ConnsOutbound:   limit.ConnsOutbound,
					FD:              limit.FD,
				},
			}
		} else {
			newLimit = &rcmgr.StaticLimit{
				Memory: limit.Memory,
				BaseLimit: rcmgr.BaseLimit{
					Streams:         limit.Streams,
					StreamsInbound:  limit.StreamsInbound,
					StreamsOutbound: limit.StreamsOutbound,
					Conns:           limit.Conns,
					ConnsInbound:    limit.ConnsInbound,
					ConnsOutbound:   limit.ConnsOutbound,
					FD:              limit.FD,
				},
			}
		}

		limiter.SetLimit(newLimit)
		return nil
	}

	switch {
	case scope == config.ResourceMgrSystemScope:
		err := mgr.ViewSystem(func(s network.ResourceScope) error {
			return setLimit(s)
		})
		return err

	case scope == config.ResourceMgrTransientScope:
		err := mgr.ViewTransient(func(s network.ResourceScope) error {
			return setLimit(s)
		})
		return err

	case strings.HasPrefix(scope, config.ResourceMgrServiceScopePrefix):
		svc := scope[4:]
		err := mgr.ViewService(svc, func(s network.ServiceScope) error {
			return setLimit(s)
		})
		return err

	case strings.HasPrefix(scope, config.ResourceMgrProtocolScopePrefix):
		proto := scope[6:]
		err := mgr.ViewProtocol(protocol.ID(proto), func(s network.ProtocolScope) error {
			return setLimit(s)
		})
		return err

	case strings.HasPrefix(scope, config.ResourceMgrPeerScopePrefix):
		p := scope[5:]
		pid, err := peer.Decode(p)
		if err != nil {
			return fmt.Errorf("invalid peer ID: %q: %w", p, err)
		}
		err = mgr.ViewPeer(pid, func(s network.PeerScope) error {
			return setLimit(s)
		})
		return err

	default:
		return fmt.Errorf("invalid scope %q", scope)
	}
}

var (
	rcmgrConnAllowed         *prometheus.CounterVec
	rcmgrConnBlocked         *prometheus.CounterVec
	rcmgrStreamAllowed       *prometheus.CounterVec
	rcmgrStreamBlocked       *prometheus.CounterVec
	rcmgrPeerAllowed         prometheus.Counter
	rcmgrPeerBlocked         prometheus.Counter
	rcmgrProtocolAllowed     prometheus.Counter
	rcmgrProtocolBlocked     prometheus.Counter
	rcmgrProtocolPeerBlocked prometheus.Counter
	rcmgrServiceAllowed      prometheus.Counter
	rcmgrServiceBlocked      prometheus.Counter
	rcmgrServicePeerBlocked  prometheus.Counter
	rcmgrMemoryAllowed       prometheus.Counter
	rcmgrMemoryBlocked       prometheus.Counter
)

func init() {
	const (
		direction = "direction"
		usesFD    = "usesFD"
	)

	rcmgrConnAllowed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_conns_allowed_total",
			Help: "allowed connections",
		},
		[]string{direction, usesFD},
	)
	prometheus.MustRegister(rcmgrConnAllowed)

	rcmgrConnBlocked = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_conns_blocked_total",
			Help: "blocked connections",
		},
		[]string{direction, usesFD},
	)
	prometheus.MustRegister(rcmgrConnBlocked)

	rcmgrStreamAllowed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_streams_allowed_total",
			Help: "allowed streams",
		},
		[]string{direction},
	)
	prometheus.MustRegister(rcmgrStreamAllowed)

	rcmgrStreamBlocked = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_streams_blocked_total",
			Help: "blocked streams",
		},
		[]string{direction},
	)
	prometheus.MustRegister(rcmgrStreamBlocked)

	rcmgrPeerAllowed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "libp2p_rcmgr_peers_allowed_total",
		Help: "allowed peers",
	})
	prometheus.MustRegister(rcmgrPeerAllowed)

	rcmgrPeerBlocked = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "libp2p_rcmgr_peer_blocked_total",
		Help: "blocked peers",
	})
	prometheus.MustRegister(rcmgrPeerBlocked)

	rcmgrProtocolAllowed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "libp2p_rcmgr_protocols_allowed_total",
		Help: "allowed streams attached to a protocol",
	})
	prometheus.MustRegister(rcmgrProtocolAllowed)

	rcmgrProtocolBlocked = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "libp2p_rcmgr_protocols_blocked_total",
		Help: "blocked streams attached to a protocol",
	})
	prometheus.MustRegister(rcmgrProtocolBlocked)

	rcmgrProtocolPeerBlocked = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "libp2p_rcmgr_protocols_for_peer_blocked_total",
		Help: "blocked streams attached to a protocol for a specific peer",
	})
	prometheus.MustRegister(rcmgrProtocolPeerBlocked)

	rcmgrServiceAllowed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "libp2p_rcmgr_services_allowed_total",
		Help: "allowed streams attached to a service",
	})
	prometheus.MustRegister(rcmgrServiceAllowed)

	rcmgrServiceBlocked = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "libp2p_rcmgr_services_blocked_total",
		Help: "blocked streams attached to a service",
	})
	prometheus.MustRegister(rcmgrServiceBlocked)

	rcmgrServicePeerBlocked = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "libp2p_rcmgr_service_for_peer_blocked_total",
		Help: "blocked streams attached to a service for a specific peer",
	})
	prometheus.MustRegister(rcmgrServicePeerBlocked)

	rcmgrMemoryAllowed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "libp2p_rcmgr_memory_allocations_allowed_total",
		Help: "allowed memory allocations",
	})
	prometheus.MustRegister(rcmgrMemoryAllowed)

	rcmgrMemoryBlocked = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "libp2p_rcmgr_memory_allocations_blocked_total",
		Help: "blocked memory allocations",
	})
	prometheus.MustRegister(rcmgrMemoryBlocked)
}

type rcmgrMetrics struct{}

func getDirection(d network.Direction) string {
	switch d {
	default:
		return ""
	case network.DirInbound:
		return "inbound"
	case network.DirOutbound:
		return "outbound"
	}
}

func (r rcmgrMetrics) AllowConn(dir network.Direction, usefd bool) {
	rcmgrConnAllowed.WithLabelValues(getDirection(dir), strconv.FormatBool(usefd)).Add(1)
}

func (r rcmgrMetrics) BlockConn(dir network.Direction, usefd bool) {
	rcmgrConnBlocked.WithLabelValues(getDirection(dir), strconv.FormatBool(usefd)).Add(1)
}

func (r rcmgrMetrics) AllowStream(_ peer.ID, dir network.Direction) {
	rcmgrStreamAllowed.WithLabelValues(getDirection(dir)).Add(1)
}

func (r rcmgrMetrics) BlockStream(_ peer.ID, dir network.Direction) {
	rcmgrStreamBlocked.WithLabelValues(getDirection(dir)).Add(1)
}

func (r rcmgrMetrics) AllowPeer(_ peer.ID) {
	rcmgrPeerAllowed.Add(1)
}

func (r rcmgrMetrics) BlockPeer(_ peer.ID) {
	rcmgrPeerBlocked.Add(1)
}

func (r rcmgrMetrics) AllowProtocol(_ protocol.ID) {
	rcmgrProtocolAllowed.Add(1)
}

func (r rcmgrMetrics) BlockProtocol(_ protocol.ID) {
	rcmgrProtocolBlocked.Add(1)
}

func (r rcmgrMetrics) BlockProtocolPeer(_ protocol.ID, _ peer.ID) {
	rcmgrProtocolPeerBlocked.Add(1)
}

func (r rcmgrMetrics) AllowService(svc string) {
	rcmgrServiceAllowed.Add(1)
}

func (r rcmgrMetrics) BlockService(svc string) {
	rcmgrServiceBlocked.Add(1)
}

func (r rcmgrMetrics) BlockServicePeer(_ string, _ peer.ID) {
	rcmgrServicePeerBlocked.Add(1)
}

func (r rcmgrMetrics) AllowMemory(_ int) {
	rcmgrMemoryAllowed.Add(1)
}

func (r rcmgrMetrics) BlockMemory(_ int) {
	rcmgrMemoryBlocked.Add(1)
}
