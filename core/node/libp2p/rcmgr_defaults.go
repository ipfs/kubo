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
// libp2p's resource manager provides tremendous flexibility but also adds a lot of complexity.
// The intent of the default config here is to provide good defaults,
// and where the defaults aren't good enough,
// to expose a good set of higher-level "knobs" to users to satisfy most use cases
// without requiring users to wade into all the intricacies of libp2p's resource manager.
//
// The inputs one can specify in SwarmConfig are:
//   - cfg.ResourceMgr.MaxMemory:  This is the max amount of memory in bytes to allow libp2p to use.
//     libp2p's resource manager will prevent additional resource creation while this limit is hit.
//     If this value isn't specified, 1/8th of the total system memory is used.
//   - cfg.ResourceMgr.MaxFileDescriptors:  This is the maximum number of file descriptors to allow libp2p to use.
//     libp2p's resource manager will prevent additional file descriptor consumption while this limit is hit.
//     If this value isn't specified, the maximum between 1/2 of system FD limit and 4096 is used.
//   - Swarm.ConnMgr.HighWater: If a connection manager is specified, libp2p's resource manager
//     will allow 2x more connections than the HighWater mark
//     so the connection manager has "space and time" to close "least useful" connections.
//
// With these inputs defined, limits are created at the system, transient, and peer scopes.
// Other scopes are ignored (by being set to infinity).
// The reason these scopes are chosen is because:
//   - system - This gives us the coarse-grained control we want so we can reason about the system as a whole.
//     It is the backstop, and allows us to reason about resource consumption more easily
//     since don't have think about the interaction of many other scopes.
//   - transient - Limiting connections that are in process of being established provides backpressure so not too much work queues up.
//   - peer - The peer scope doesn't protect us against intentional DoS attacks.
//     It's just as easy for an attacker to send 100 requests/second with 1 peerId vs. 10 requests/second with 10 peers.
//     We are reliant on the system scope for protection here in the malicious case.
//     The reason for having a peer scope is to protect against unintentional DoS attacks
//     (e.g., bug in a peer which is causing it to "misbehave").
//     In the unintional case, we want to make sure a "misbehaving" node doesn't consume more resources than necessary.
//
// Within these scopes, limits are just set on memory, FD, and inbound connections/streams.
// Limits are set based on the inputs above.
// We trust this node to behave properly and thus ignore outbound connection/stream limits.
// We apply any limits that libp2p has for its protocols/services
// since we assume libp2p knows best here.
//
// This leaves 3 levels of resource management protection:
//  1. The user who does nothing and uses defaults - In this case they get some sane defaults
//     based on the amount of memory and file descriptors their system has.
//     This should protect the node from many attacks.
//  2. Slightly more advanced user - They can tweak the above by passing in config on
//     maxMemory, maxFD, or maxConns with Swarm.HighWater.ConnMgr.
//  3. Power user - They specify all the limits they want set via Swarm.ResourceMgr.Limits
//     and we don't do any defaults/overrides. We pass that config blindly into libp2p resource manager.
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
			// Note that the limit gets adjusted below if "cfg.ConnMgr.HighWater" is set.
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

		// Just go with what libp2p does
		TransientBaseLimit:     rcmgr.DefaultLimits.TransientBaseLimit,
		TransientLimitIncrease: rcmgr.DefaultLimits.TransientLimitIncrease,

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

	// If a high water mark is set:
	if cfg.ConnMgr.Type == "basic" {
		// set the connection limit higher than high water mark so that the ConnMgr has "space and time" to close "least useful" connections.
		defaultLimitConfig.System.Conns = 2 * cfg.ConnMgr.HighWater
		log.Info("adjusted default resource manager System.Conns limits to match ConnMgr.HighWater value of %s", cfg.ConnMgr.HighWater)
	}

	return defaultLimitConfig, nil
}
