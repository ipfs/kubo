package libp2p

import (
	"math"

	"github.com/ipfs/kubo/config"
	"github.com/libp2p/go-libp2p"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
)

// We are doing some magic when parsing config files (we are using a map[string]interface{} to compare config files).
// When you don't have a type the JSON Parse function cast numbers to float64 by default,
// losing precision when writing the final number. So if we use math.MaxInt as our infinite number,
// after writing the config file we will have 9223372036854776000 instead of 9223372036854775807,
// making the parsing process fail.
const bigEnough = math.MaxInt / 2

var infiniteBaseLimit = rcmgr.BaseLimit{
	Streams:         bigEnough,
	StreamsInbound:  bigEnough,
	StreamsOutbound: bigEnough,
	Conns:           bigEnough,
	ConnsInbound:    bigEnough,
	ConnsOutbound:   bigEnough,
	FD:              bigEnough,
	Memory:          bigEnough,
}

// This file defines implicit limit defaults used when Swarm.ResourceMgr.Enabled

// adjustedDefaultLimits allows for tweaking defaults based on external factors,
// such as values in Swarm.ConnMgr.HiWater config.
func adjustedDefaultLimits(cfg config.SwarmConfig) rcmgr.LimitConfig {
	defaultLimits := rcmgr.ScalingLimitConfig{
		SystemBaseLimit: rcmgr.BaseLimit{
			Memory: rcmgr.DefaultLimits.SystemBaseLimit.Memory,
			FD:     rcmgr.DefaultLimits.SystemBaseLimit.FD,

			Conns:         bigEnough,                                        // just limit on the inbound
			ConnsInbound:  rcmgr.DefaultLimits.SystemBaseLimit.ConnsInbound, // same as libp2p default
			ConnsOutbound: bigEnough,

			// Don't limit streams.  Rely on peer and transient limits.
			Streams:         bigEnough,
			StreamsInbound:  bigEnough,
			StreamsOutbound: bigEnough,
		},

		// Just go with what libp2p does
		TransientBaseLimit:     rcmgr.DefaultLimits.TransientBaseLimit,
		TransientLimitIncrease: rcmgr.DefaultLimits.TransientLimitIncrease,

		// Lets get out of the way of the allow list functionality.
		// If someone specified "Swarm.ResourceMgr.Allowlist" we should let it go through.
		AllowlistedSystemBaseLimit:    infiniteBaseLimit,
		AllowlistedTransientBaseLimit: infiniteBaseLimit,

		// Keep it simple by not having Service, ServicePeer, Protocol, ProtocolPeer, Conn, or Stream limits.
		ServiceBaseLimit:      infiniteBaseLimit,
		ServicePeerBaseLimit:  infiniteBaseLimit,
		ProtocolBaseLimit:     infiniteBaseLimit,
		ProtocolPeerBaseLimit: infiniteBaseLimit,
		ConnBaseLimit:         infiniteBaseLimit,
		StreamBaseLimit:       infiniteBaseLimit,

		// Limit connections per peer. Really important to mitigate flooding attacks from a peer.
		PeerBaseLimit: rcmgr.BaseLimit{
			Streams:         bigEnough,
			StreamsOutbound: bigEnough,
			StreamsInbound:  rcmgr.DefaultLimits.PeerBaseLimit.StreamsInbound,
			Conns:           bigEnough,
			ConnsInbound:    rcmgr.DefaultLimits.PeerBaseLimit.ConnsInbound,
			ConnsOutbound:   bigEnough,
			FD:              rcmgr.DefaultLimits.PeerBaseLimit.FD,
			Memory:          rcmgr.DefaultLimits.PeerBaseLimit.Memory,
		},
	}

	libp2p.SetDefaultServiceLimits(&defaultLimits)

	defaultLimitConfig := defaultLimits.AutoScale()

	// If a high water mark is set:
	if cfg.ConnMgr.Type == "basic" {
		// set the connection limit higher than high water mark so that the ConnMgr has "space and time" to close "least useful" connections.
		defaultLimitConfig.System.Conns = 2 * cfg.ConnMgr.HighWater
		log.Info("adjusted default resource manager System.Conns limits to match ConnMgr.HighWater value of %s", cfg.ConnMgr.HighWater)
	}

	return defaultLimitConfig
}
