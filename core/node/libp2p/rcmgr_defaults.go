package libp2p

import (
	"math/bits"

	config "github.com/ipfs/go-ipfs/config"
	rcmgr "github.com/libp2p/go-libp2p-resource-manager"
)

// This file defines implicit limit defaults used when Swarm.ResourceMgr.Enabled

// adjustedDefaultLimits allows for tweaking defaults based on external factors,
// such as values in Swarm.ConnMgr.HiWater config.
func adjustedDefaultLimits(cfg config.SwarmConfig) rcmgr.DefaultLimitConfig {
	// Return to use unmodified static limits based on values from go-libp2p 0.18
	// return defaultLimits

	// Adjust limits
	// (based on https://github.com/filecoin-project/lotus/pull/8318/files)
	// - give it more memory, up to 4G, min of 1G
	// - if Swarm.ConnMgr.HighWater is too high, adjust Conn/FD/Stream limits
	defaultLimits := rcmgr.DefaultLimits.WithSystemMemory(.125, 1<<30, 4<<30)

	// Do we need to adjust due to Swarm.ConnMgr.HighWater?
	if cfg.ConnMgr.Type == "basic" {
		maxconns := cfg.ConnMgr.HighWater
		if 2*maxconns > defaultLimits.SystemBaseLimit.ConnsInbound {
			// adjust conns to 2x to allow for two conns per peer (TCP+QUIC)
			defaultLimits.SystemBaseLimit.ConnsInbound = logScale(2 * maxconns)
			defaultLimits.SystemBaseLimit.ConnsOutbound = logScale(2 * maxconns)
			defaultLimits.SystemBaseLimit.Conns = logScale(4 * maxconns)

			defaultLimits.SystemBaseLimit.StreamsInbound = logScale(16 * maxconns)
			defaultLimits.SystemBaseLimit.StreamsOutbound = logScale(64 * maxconns)
			defaultLimits.SystemBaseLimit.Streams = logScale(64 * maxconns)

			if 2*maxconns > defaultLimits.SystemBaseLimit.FD {
				defaultLimits.SystemBaseLimit.FD = logScale(2 * maxconns)
			}

			defaultLimits.ServiceBaseLimit.StreamsInbound = logScale(8 * maxconns)
			defaultLimits.ServiceBaseLimit.StreamsOutbound = logScale(32 * maxconns)
			defaultLimits.ServiceBaseLimit.Streams = logScale(32 * maxconns)

			defaultLimits.ProtocolBaseLimit.StreamsInbound = logScale(8 * maxconns)
			defaultLimits.ProtocolBaseLimit.StreamsOutbound = logScale(32 * maxconns)
			defaultLimits.ProtocolBaseLimit.Streams = logScale(32 * maxconns)
		}
	}

	return defaultLimits
}

func logScale(val int) int {
	bitlen := bits.Len(uint(val))
	return 1 << bitlen
}
