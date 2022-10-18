package libp2p

// maybe there are other imports that can be cleaned up?
import (
	"encoding/json"
	"fmt"
	"math/bits"
	"os"
	"strings"

	"github.com/ipfs/kubo/config"
	"github.com/libp2p/go-libp2p"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
)

// This file defines implicit limit defaults used when Swarm.ResourceMgr.Enabled

// adjustedDefaultLimits allows for tweaking defaults based on external factors,
// such as values in Swarm.ConnMgr.HiWater config.
func adjustedDefaultLimits(cfg config.SwarmConfig) rcmgr.LimitConfig {
	// Run checks to avoid introducing regressions
	if os.Getenv("IPFS_CHECK_RCMGR_DEFAULTS") != "" {
		checkImplicitDefaults()
	}
	
	// Get or calculate maxMemory and maxFD 
	// These can come in from config.
	// If a user doesn't set these, then we can assume the same defaults as libp2p uses for its "AutoScale" function.
	// 1. 1/8th of system memory
	// 2. Math.max(1/2 of system FD limit, 4096)
	// I don't know the right way to write this go code but I assume we can lift from
	// lifting from https://github.com/libp2p/go-libp2p/blob/master/p2p/host/resource-manager/limit_defaults.go#L309-L310
	// Note because we are determining the maxFD and maxMemory, we are going to call the "Scale" function and not "AutoScale" below.
	
	var noLimitIncrease = BaseLimitIncrease {
		ConnsInbound:    0,
		ConnsOutbound:   0,
		Conns:           0,
		StreamsInbound:  0,
		StreamsOutbound: 0,
		Streams:         0,
		Memory:          0,
		FDFraction:      0,
	}
	
	var infiniteBaseLimit = BaseLimit{
		Streams:         math.MaxInt,
		StreamsInbound:  math.MaxInt,
		StreamsOutbound: math.MaxInt,
		Conns:           math.MaxInt,
		ConnsInbound:    math.MaxInt,
		ConnsOutbound:   math.MaxInt,
		FD:              math.MaxInt,
		Memory:          math.MaxInt64,
	}
	
	// I assume I'm doing inccorect Go code here and not always using "=" vs. ":=" right.
	scalingLimitConfig = ScalingLimitConfig {
		SystemBaseLimit: BaseLimit{
			// Use the input parameters on memory and FD
			Memory:          maxMemory,
			FD:              maxFD,

			Conns:           math.MaxInt, // just limit on the inbound
			ConnsInbound:    rcmgr.DefaultLimits.SystemBaseLimit.ConnsInbound, // same as libp2p default
			ConnsOutbound:   math.MaxInt,
			// Don't limit streams.  Rely on connection and memory limits.
			Streams:         math.MaxInt,
			StreamsInbound:  math.MaxInt,
			StreamsOutbound: math.MaxInt,
		},
		SystemLimitIncrease: noLimitIncrease,

		// Just go with what libp2p does
		// We could simplify this to only worry about Memory, FD, and ConnsInbound.
		// I figure we shouldn't have resources in Transient state long so it's fine to have some limits here 
		// because if they are getting hit, it's a problem
		TransientBaseLimit: rcmgr.DefaultLimits.TransientBaseLimit,
		TransientLimitIncrease: rcmgr.DefaultLimits.BaseLimitIncrease,

		// Lets get out of the way of the allow list functionality.
		// If someone specified "Swarm.ResourceMgr.Allowlist" we should let it go through.
		AllowlistedSystemBaseLimit: infiniteBaseLimit,
		AllowlistedSystemLimitIncrease: noLimitIncrease,
		AllowlistedTransientBaseLimit: infiniteBaseLimit,
		AllowlistedTransientLimitIncrease: noLimitIncrease,

		// Keep it simple by not having Service, ServicePeer, Protocol, ProtocolPeer, Peer, Conn, or Stream limits.
		ServiceBaseLimit: infiniteBaseLimit,
		ServiceLimitIncrease: noLimitIncrease,
		ServicePeerBaseLimit: infiniteBaseLimit,
		ServicePeerLimitIncrease: noLimitIncrease,
		ProtocolBaseLimit: infiniteBaseLimit,
		ProtocolLimitIncrease: noLimitIncrease,
		ProtocolPeerBaseLimit: infiniteBaseLimit,
		ProtocolPeerLimitIncrease: noLimitIncrease,
		PeerBaseLimit: infiniteBaseLimit,
		PeerLimitIncrease: noLimitIncrease,
		ConnBaseLimit: infiniteBaseLimit,
		StreamBaseLimit: infiniteBaseLimit,
	}
	
	// Whatever limits libp2p has specifically tuned for its protocols/services we'll apply.
	libp2p.SetDefaultServiceLimits(&scalingLimitConfig)
	
	defaultLimitConfig = scalingLimitConfig.Scale(maxMemory, maxFD)

	// If a high water mark is set, 
	if cfg.ConnMgr.Type == "basic" {
		// set the connection limit higher than high water mark so that the ConnMgr has "space and time" to close "least useful" connections.
		defaultLimitConfig.System.Conns = 2*cfg.ConnMgr.HighWater 
		log.Info("adjusted default resource manager System.Conns limits to match ConnMgr.HighWater value of %s", cfg.ConnMgr.HighWater)
	}

	return defaultLimitConfig
}
