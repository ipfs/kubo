package libp2p

import (
	"math"

	"github.com/dustin/go-humanize"
	"github.com/libp2p/go-libp2p"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/pbnjay/memory"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/node/libp2p/fd"
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

var noLimitIncrease = rcmgr.BaseLimitIncrease{
	ConnsInbound:    0,
	ConnsOutbound:   0,
	Conns:           0,
	StreamsInbound:  0,
	StreamsOutbound: 0,
	Streams:         0,
	Memory:          0,
	FDFraction:      0,
}

// This file defines implicit limit defaults used when Swarm.ResourceMgr.Enabled

// createDefaultLimitConfig creates LimitConfig to pass to libp2p's resource manager.
// The defaults follow the documentation in docs/config.md.
// Any changes in the logic here should be reflected there.
func createDefaultLimitConfig(cfg config.SwarmConfig) (rcmgr.LimitConfig, error) {
	maxMemoryDefaultString := humanize.Bytes(uint64(memory.TotalMemory()) / 8)
	maxMemoryString := cfg.ResourceMgr.MaxMemory.WithDefault(maxMemoryDefaultString)
	maxMemory, err := humanize.ParseBytes(maxMemoryString)
	if err != nil {
		return rcmgr.LimitConfig{}, err
	}

	numFD := cfg.ResourceMgr.MaxFileDescriptors.WithDefault(int64(fd.GetNumFDs()) / 2)

	scalingLimitConfig := rcmgr.ScalingLimitConfig{
		SystemBaseLimit: rcmgr.BaseLimit{
			Memory: int64(maxMemory),
			FD:     int(numFD),

			// By default, we just limit connections on the inbound side.
			Conns:         bigEnough,
			ConnsInbound:  rcmgr.DefaultLimits.SystemBaseLimit.ConnsInbound, // same as libp2p default
			ConnsOutbound: bigEnough,

			// We limit streams since they not only take up memory and CPU.
			// The Memory limit protects us on the memory side,
			// but a StreamsInbound limit helps protect against unbound CPU consumption from stream processing.
			Streams:         bigEnough,
			StreamsInbound:  rcmgr.DefaultLimits.SystemBaseLimit.StreamsInbound,
			StreamsOutbound: bigEnough,
		},
		// Most limits don't see an increase because they're already infinite/bigEnough or at their max value.
		// The values that should scale based on the amount of memory allocated to libp2p need to increase accordingly.
		SystemLimitIncrease: rcmgr.BaseLimitIncrease{
			Memory:     rcmgr.DefaultLimits.SystemLimitIncrease.Memory,
			FDFraction: rcmgr.DefaultLimits.SystemLimitIncrease.FDFraction,

			Conns:         0,
			ConnsInbound:  rcmgr.DefaultLimits.SystemLimitIncrease.ConnsInbound,
			ConnsOutbound: 0,

			Streams:         0,
			StreamsInbound:  rcmgr.DefaultLimits.SystemLimitIncrease.StreamsInbound,
			StreamsOutbound: 0,
		},

		TransientBaseLimit: rcmgr.BaseLimit{
			Memory: rcmgr.DefaultLimits.TransientBaseLimit.Memory,
			FD:     rcmgr.DefaultLimits.TransientBaseLimit.FD,

			Conns:         bigEnough,
			ConnsInbound:  rcmgr.DefaultLimits.TransientBaseLimit.ConnsInbound,
			ConnsOutbound: bigEnough,

			Streams:         bigEnough,
			StreamsInbound:  rcmgr.DefaultLimits.TransientBaseLimit.StreamsInbound,
			StreamsOutbound: bigEnough,
		},

		TransientLimitIncrease: rcmgr.BaseLimitIncrease{
			Memory:     rcmgr.DefaultLimits.TransientLimitIncrease.Memory,
			FDFraction: rcmgr.DefaultLimits.TransientLimitIncrease.FDFraction,

			Conns:         0,
			ConnsInbound:  rcmgr.DefaultLimits.TransientLimitIncrease.ConnsInbound,
			ConnsOutbound: 0,

			Streams:         0,
			StreamsInbound:  rcmgr.DefaultLimits.TransientLimitIncrease.StreamsInbound,
			StreamsOutbound: 0,
		},

		// Lets get out of the way of the allow list functionality.
		// If someone specified "Swarm.ResourceMgr.Allowlist" we should let it go through.
		AllowlistedSystemBaseLimit:     infiniteBaseLimit,
		AllowlistedSystemLimitIncrease: noLimitIncrease,

		AllowlistedTransientBaseLimit:     infiniteBaseLimit,
		AllowlistedTransientLimitIncrease: noLimitIncrease,

		// Keep it simple by not having Service, ServicePeer, Protocol, ProtocolPeer, Conn, or Stream limits.
		ServiceBaseLimit:     infiniteBaseLimit,
		ServiceLimitIncrease: noLimitIncrease,

		ServicePeerBaseLimit:     infiniteBaseLimit,
		ServicePeerLimitIncrease: noLimitIncrease,

		ProtocolBaseLimit:     infiniteBaseLimit,
		ProtocolLimitIncrease: noLimitIncrease,

		ProtocolPeerBaseLimit:     infiniteBaseLimit,
		ProtocolPeerLimitIncrease: noLimitIncrease,

		ConnBaseLimit:     infiniteBaseLimit,
		ConnLimitIncrease: noLimitIncrease,

		StreamBaseLimit:     infiniteBaseLimit,
		StreamLimitIncrease: noLimitIncrease,

		// Limit the resources consumed by a peer.
		// This doesn't protect us against intentional DoS attacks since an attacker can easily spin up multiple peers.
		// We specify this limit against unintentional DoS attacks (e.g., a peer has a bug and is sending too much traffic intentionally).
		// In that case we want to keep that peer's resource consumption contained.
		// To keep this simple, we only constrain inbound connections and streams.
		PeerBaseLimit: rcmgr.BaseLimit{
			Memory:          bigEnough,
			FD:              bigEnough,
			Conns:           bigEnough,
			ConnsInbound:    rcmgr.DefaultLimits.PeerBaseLimit.ConnsInbound,
			ConnsOutbound:   bigEnough,
			Streams:         bigEnough,
			StreamsInbound:  rcmgr.DefaultLimits.PeerBaseLimit.StreamsInbound,
			StreamsOutbound: bigEnough,
		},
		// Most limits don't see an increase because they're already infinite/bigEnough.
		// The values that should scale based on the amount of memory allocated to libp2p need to increase accordingly.
		PeerLimitIncrease: rcmgr.BaseLimitIncrease{
			Memory:          0,
			FDFraction:      0,
			Conns:           0,
			ConnsInbound:    rcmgr.DefaultLimits.PeerLimitIncrease.ConnsInbound,
			ConnsOutbound:   0,
			Streams:         0,
			StreamsInbound:  rcmgr.DefaultLimits.PeerLimitIncrease.StreamsInbound,
			StreamsOutbound: 0,
		},
	}

	// Whatever limits libp2p has specifically tuned for its protocols/services we'll apply.
	libp2p.SetDefaultServiceLimits(&scalingLimitConfig)

	defaultLimitConfig := scalingLimitConfig.Scale(int64(maxMemory), int(numFD))

	return defaultLimitConfig, nil
}
