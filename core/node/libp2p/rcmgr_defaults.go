package libp2p

import (
	"fmt"

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
// making the parsing process fail. Setting 1e9 (1000000000) as "no limit" value. It also avoids to overflow on 32 bit architectures.
const bigEnough = 1e9

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
// The defaults follow the documentation in docs/libp2p-resource-management.md.
// Any changes in the logic here should be reflected there.
func createDefaultLimitConfig(cfg config.SwarmConfig) (rcmgr.LimitConfig, error) {
	maxMemoryDefaultString := humanize.Bytes(uint64(memory.TotalMemory()) / 2)
	maxMemoryString := cfg.ResourceMgr.MaxMemory.WithDefault(maxMemoryDefaultString)
	maxMemory, err := humanize.ParseBytes(maxMemoryString)
	if err != nil {
		return rcmgr.LimitConfig{}, err
	}

	maxMemoryMB := maxMemory / (1024 * 1024)
	maxFD := int(cfg.ResourceMgr.MaxFileDescriptors.WithDefault(int64(fd.GetNumFDs()) / 2))

	// We want to see this message on startup, that's why we are using fmt instead of log.
	fmt.Printf(`
Computing default go-libp2p Resource Manager limits based on:
    - 'Swarm.ResourceMgr.MaxMemory': %q
    - 'Swarm.ResourceMgr.MaxFileDescriptors': %d

Applying any user-supplied overrides on top.
Run 'ipfs swarm limit all' to see the resulting limits.

`, maxMemoryString, maxFD)

	// At least as of 2023-01-25, it's possible to open a connection that
	// doesn't ask for any memory usage with the libp2p Resource Manager/Accountant
	// (see https://github.com/libp2p/go-libp2p/issues/2010#issuecomment-1404280736).
	// As a result, we can't curretly rely on Memory limits to full protect us.
	// Until https://github.com/libp2p/go-libp2p/issues/2010 is addressed,
	// we take a proxy now of restricting to 1 inbound connection per MB.
	// Note: this is more generous than go-libp2p's default autoscaled limits which do
	// 64 connections per 1GB
	// (see https://github.com/libp2p/go-libp2p/blob/master/p2p/host/resource-manager/limit_defaults.go#L357 ).
	systemConnsInbound := int(1 * maxMemoryMB)

	scalingLimitConfig := rcmgr.ScalingLimitConfig{
		SystemBaseLimit: rcmgr.BaseLimit{
			Memory: int64(maxMemory),
			FD:     maxFD,

			// By default, we just limit connections on the inbound side.
			Conns:         bigEnough,
			ConnsInbound:  systemConnsInbound,
			ConnsOutbound: bigEnough,

			Streams:         bigEnough,
			StreamsInbound:  bigEnough,
			StreamsOutbound: bigEnough,
		},
		SystemLimitIncrease: noLimitIncrease,

		// Transient connections won't cause any memory to accounted for by the resource manager.
		// Only established connections do.
		// As a result, we can't rely on System.Memory to protect us from a bunch of transient connection being opened.
		// We limit the same values as the System scope, but only allow the Transient scope to take 25% of what is allowed for the System scope.
		TransientBaseLimit: rcmgr.BaseLimit{
			Memory: int64(maxMemory / 4),
			FD:     maxFD / 4,

			Conns:         bigEnough,
			ConnsInbound:  systemConnsInbound / 4,
			ConnsOutbound: bigEnough,

			Streams:         bigEnough,
			StreamsInbound:  bigEnough,
			StreamsOutbound: bigEnough,
		},

		TransientLimitIncrease: noLimitIncrease,

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

	defaultLimitConfig := scalingLimitConfig.Scale(int64(maxMemory), maxFD)

	// Simple checks to overide autoscaling ensuring limits make sense versus the connmgr values.
	// There are ways to break this, but this should catch most problems already.
	// We might improve this in the future.
	// See: https://github.com/ipfs/kubo/issues/9545
	if cfg.ConnMgr.Type.WithDefault(config.DefaultConnMgrType) != "none" {
		maxInboundConns := int64(defaultLimitConfig.System.ConnsInbound)
		if connmgrHighWaterTimesTwo := cfg.ConnMgr.HighWater.WithDefault(config.DefaultConnMgrHighWater) * 2; maxInboundConns < connmgrHighWaterTimesTwo {
			maxInboundConns = connmgrHighWaterTimesTwo
		}

		if maxInboundConns < config.DefaultResourceMgrMinInboundConns {
			maxInboundConns = config.DefaultResourceMgrMinInboundConns
		}

		// Scale System.StreamsInbound as well, but use the existing ratio of StreamsInbound to ConnsInbound
		defaultLimitConfig.System.StreamsInbound = int(maxInboundConns * int64(defaultLimitConfig.System.StreamsInbound) / int64(defaultLimitConfig.System.ConnsInbound))
		defaultLimitConfig.System.ConnsInbound = int(maxInboundConns)
	}

	return defaultLimitConfig, nil
}
