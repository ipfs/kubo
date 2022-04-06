package libp2p

import (
	"math/bits"

	config "github.com/ipfs/go-ipfs/config"
	"github.com/libp2p/go-libp2p-core/protocol"
	rcmgr "github.com/libp2p/go-libp2p-resource-manager"
	"github.com/libp2p/go-libp2p/p2p/host/autonat"
	relayv1 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv1/relay"
	circuit "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/proto"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
)

// This file defines implicit limit defaults used when Swarm.ResourceMgr.Enabled
// We keep vendored copy to ensure go-ipfs is not impacted when go-libp2p decides
// to change defaults in any of the future releases.

// adjustedDefaultLimits allows for tweaking defaults based on external factors,
// such as values in Swarm.ConnMgr.HiWater config.
func adjustedDefaultLimits(cfg config.SwarmConfig) rcmgr.DefaultLimitConfig {

	// Return to use unmodified static limits based on values from go-libp2p 0.18
	// return defaultLimits

	// Adjust limits
	// (based on https://github.com/filecoin-project/lotus/pull/8318/files)
	// - give it more memory, up to 4G, min of 1G
	// - if Swarm.ConnMgr.HighWater is too high, adjust Conn/FD/Stream limits
	defaultLimits := staticDefaultLimits.WithSystemMemory(.125, 1<<30, 4<<30)

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

// defaultLimits are the limits used by the default rcmgr limiter constructors.
// This is  a vendored copy of
// https://github.com/libp2p/go-libp2p-resource-manager/blob/v0.1.5/limit_defaults.go#L49
var staticDefaultLimits = rcmgr.DefaultLimitConfig{
	SystemBaseLimit: rcmgr.BaseLimit{
		StreamsInbound:  4096,
		StreamsOutbound: 16384,
		Streams:         16384,
		ConnsInbound:    256,
		ConnsOutbound:   1024,
		Conns:           1024,
		FD:              512,
	},

	SystemMemory: rcmgr.MemoryLimit{
		MemoryFraction: 0.125,
		MinMemory:      128 << 20,
		MaxMemory:      1 << 30,
	},

	TransientBaseLimit: rcmgr.BaseLimit{
		StreamsInbound:  128,
		StreamsOutbound: 512,
		Streams:         512,
		ConnsInbound:    32,
		ConnsOutbound:   128,
		Conns:           128,
		FD:              128,
	},

	TransientMemory: rcmgr.MemoryLimit{
		MemoryFraction: 1,
		MinMemory:      64 << 20,
		MaxMemory:      64 << 20,
	},

	ServiceBaseLimit: rcmgr.BaseLimit{
		StreamsInbound:  2048,
		StreamsOutbound: 8192,
		Streams:         8192,
	},

	ServiceMemory: rcmgr.MemoryLimit{
		MemoryFraction: 0.125 / 4,
		MinMemory:      64 << 20,
		MaxMemory:      256 << 20,
	},

	ServicePeerBaseLimit: rcmgr.BaseLimit{
		StreamsInbound:  256,
		StreamsOutbound: 512,
		Streams:         512,
	},

	ServicePeerMemory: rcmgr.MemoryLimit{
		MemoryFraction: 0.125 / 16,
		MinMemory:      16 << 20,
		MaxMemory:      64 << 20,
	},

	ProtocolBaseLimit: rcmgr.BaseLimit{
		StreamsInbound:  1024,
		StreamsOutbound: 4096,
		Streams:         4096,
	},

	ProtocolMemory: rcmgr.MemoryLimit{
		MemoryFraction: 0.125 / 8,
		MinMemory:      64 << 20,
		MaxMemory:      128 << 20,
	},

	ProtocolPeerBaseLimit: rcmgr.BaseLimit{
		StreamsInbound:  128,
		StreamsOutbound: 256,
		Streams:         512,
	},

	ProtocolPeerMemory: rcmgr.MemoryLimit{
		MemoryFraction: 0.125 / 16,
		MinMemory:      16 << 20,
		MaxMemory:      64 << 20,
	},

	PeerBaseLimit: rcmgr.BaseLimit{
		StreamsInbound:  512,
		StreamsOutbound: 1024,
		Streams:         1024,
		ConnsInbound:    8,
		ConnsOutbound:   16,
		Conns:           16,
		FD:              8,
	},

	PeerMemory: rcmgr.MemoryLimit{
		MemoryFraction: 0.125 / 16,
		MinMemory:      64 << 20,
		MaxMemory:      128 << 20,
	},

	ConnBaseLimit: rcmgr.BaseLimit{
		ConnsInbound:  1,
		ConnsOutbound: 1,
		Conns:         1,
		FD:            1,
	},

	ConnMemory: 1 << 20,

	StreamBaseLimit: rcmgr.BaseLimit{
		StreamsInbound:  1,
		StreamsOutbound: 1,
		Streams:         1,
	},

	StreamMemory: 16 << 20,
}

// setDefaultServiceLimits sets the default limits for bundled libp2p services.
// This is a vendored copy of
// https://github.com/libp2p/go-libp2p/blob/v0.18.0/limits.go
func setDefaultServiceLimits(limiter *rcmgr.BasicLimiter) {
	if limiter.ServiceLimits == nil {
		limiter.ServiceLimits = make(map[string]rcmgr.Limit)
	}
	if limiter.ServicePeerLimits == nil {
		limiter.ServicePeerLimits = make(map[string]rcmgr.Limit)
	}
	if limiter.ProtocolLimits == nil {
		limiter.ProtocolLimits = make(map[protocol.ID]rcmgr.Limit)
	}
	if limiter.ProtocolPeerLimits == nil {
		limiter.ProtocolPeerLimits = make(map[protocol.ID]rcmgr.Limit)
	}

	// identify
	setServiceLimits(limiter, identify.ServiceName,
		limiter.DefaultServiceLimits.
			WithMemoryLimit(1, 4<<20, 64<<20). // max 64MB service memory
			WithStreamLimit(128, 128, 256),    // max 256 streams -- symmetric
		peerLimit(16, 16, 32))

	setProtocolLimits(limiter, identify.ID,
		limiter.DefaultProtocolLimits.WithMemoryLimit(1, 4<<20, 32<<20),
		peerLimit(16, 16, 32))
	setProtocolLimits(limiter, identify.IDPush,
		limiter.DefaultProtocolLimits.WithMemoryLimit(1, 4<<20, 32<<20),
		peerLimit(16, 16, 32))
	setProtocolLimits(limiter, identify.IDDelta,
		limiter.DefaultProtocolLimits.WithMemoryLimit(1, 4<<20, 32<<20),
		peerLimit(16, 16, 32))

	// ping
	setServiceLimits(limiter, ping.ServiceName,
		limiter.DefaultServiceLimits.
			WithMemoryLimit(1, 4<<20, 64<<20). // max 64MB service memory
			WithStreamLimit(128, 128, 128),    // max 128 streams - asymmetric
		peerLimit(2, 3, 4))
	setProtocolLimits(limiter, ping.ID,
		limiter.DefaultProtocolLimits.WithMemoryLimit(1, 4<<20, 64<<20),
		peerLimit(2, 3, 4))

	// autonat
	setServiceLimits(limiter, autonat.ServiceName,
		limiter.DefaultServiceLimits.
			WithMemoryLimit(1, 4<<20, 64<<20). // max 64MB service memory
			WithStreamLimit(128, 128, 128),    // max 128 streams - asymmetric
		peerLimit(2, 2, 2))
	setProtocolLimits(limiter, autonat.AutoNATProto,
		limiter.DefaultProtocolLimits.WithMemoryLimit(1, 4<<20, 64<<20),
		peerLimit(2, 2, 2))

	// holepunch
	setServiceLimits(limiter, holepunch.ServiceName,
		limiter.DefaultServiceLimits.
			WithMemoryLimit(1, 4<<20, 64<<20). // max 64MB service memory
			WithStreamLimit(128, 128, 256),    // max 256 streams - symmetric
		peerLimit(2, 2, 2))
	setProtocolLimits(limiter, holepunch.Protocol,
		limiter.DefaultProtocolLimits.WithMemoryLimit(1, 4<<20, 64<<20),
		peerLimit(2, 2, 2))

	// relay/v1
	setServiceLimits(limiter, relayv1.ServiceName,
		limiter.DefaultServiceLimits.
			WithMemoryLimit(1, 4<<20, 64<<20). // max 64MB service memory
			WithStreamLimit(1024, 1024, 1024), // max 1024 streams - asymmetric
		peerLimit(64, 64, 64))

	// relay/v2
	setServiceLimits(limiter, relayv2.ServiceName,
		limiter.DefaultServiceLimits.
			WithMemoryLimit(1, 4<<20, 64<<20). // max 64MB service memory
			WithStreamLimit(1024, 1024, 1024), // max 1024 streams - asymmetric
		peerLimit(64, 64, 64))

	// circuit protocols, both client and service
	setProtocolLimits(limiter, circuit.ProtoIDv1,
		limiter.DefaultProtocolLimits.
			WithMemoryLimit(1, 4<<20, 64<<20).
			WithStreamLimit(1280, 1280, 1280),
		peerLimit(128, 128, 128))
	setProtocolLimits(limiter, circuit.ProtoIDv2Hop,
		limiter.DefaultProtocolLimits.
			WithMemoryLimit(1, 4<<20, 64<<20).
			WithStreamLimit(1280, 1280, 1280),
		peerLimit(128, 128, 128))
	setProtocolLimits(limiter, circuit.ProtoIDv2Stop,
		limiter.DefaultProtocolLimits.
			WithMemoryLimit(1, 4<<20, 64<<20).
			WithStreamLimit(1280, 1280, 1280),
		peerLimit(128, 128, 128))

}

func setServiceLimits(limiter *rcmgr.BasicLimiter, svc string, limit rcmgr.Limit, peerLimit rcmgr.Limit) {
	if _, ok := limiter.ServiceLimits[svc]; !ok {
		limiter.ServiceLimits[svc] = limit
	}
	if _, ok := limiter.ServicePeerLimits[svc]; !ok {
		limiter.ServicePeerLimits[svc] = peerLimit
	}
}

func setProtocolLimits(limiter *rcmgr.BasicLimiter, proto protocol.ID, limit rcmgr.Limit, peerLimit rcmgr.Limit) {
	if _, ok := limiter.ProtocolLimits[proto]; !ok {
		limiter.ProtocolLimits[proto] = limit
	}
	if _, ok := limiter.ProtocolPeerLimits[proto]; !ok {
		limiter.ProtocolPeerLimits[proto] = peerLimit
	}
}

func peerLimit(numStreamsIn, numStreamsOut, numStreamsTotal int) rcmgr.Limit {
	return &rcmgr.StaticLimit{
		// memory: 256kb for window buffers plus some change for message buffers per stream
		Memory: int64(numStreamsTotal * (256<<10 + 16384)),
		BaseLimit: rcmgr.BaseLimit{
			StreamsInbound:  numStreamsIn,
			StreamsOutbound: numStreamsOut,
			Streams:         numStreamsTotal,
		},
	}
}
