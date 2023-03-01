package cli

import (
	"encoding/json"
	"testing"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRcmgr(t *testing.T) {
	t.Parallel()

	t.Run("Resource manager disabled", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Swarm.ResourceMgr.Enabled = config.False
		})

		node.StartDaemon()

		t.Run("swarm resources should fail", func(t *testing.T) {
			res := node.RunIPFS("swarm", "resources")
			assert.Equal(t, 1, res.ExitCode())
			assert.Contains(t, res.Stderr.String(), "missing ResourceMgr")
		})
	})

	t.Run("Node with resource manager disabled", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Swarm.ResourceMgr.Enabled = config.False
		})
		node.StartDaemon()

		t.Run("swarm resources should fail", func(t *testing.T) {
			res := node.RunIPFS("swarm", "resources")
			assert.Equal(t, 1, res.ExitCode())
			assert.Contains(t, res.Stderr.String(), "missing ResourceMgr")
		})
	})

	t.Run("Very high connmgr highwater", func(t *testing.T) {
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Swarm.ConnMgr.HighWater = config.NewOptionalInteger(1000)
		})
		node.StartDaemon()

		res := node.RunIPFS("swarm", "resources", "--enc=json")
		require.Equal(t, 0, res.ExitCode())
		limits := unmarshalLimits(t, res.Stdout.Bytes())

		rl := limits.System.ToResourceLimits()
		s := rl.Build(rcmgr.BaseLimit{})
		assert.GreaterOrEqual(t, s.ConnsInbound, 2000)
		assert.GreaterOrEqual(t, s.StreamsInbound, 2000)
	})

	t.Run("default configuration", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Swarm.ConnMgr.HighWater = config.NewOptionalInteger(1000)
		})
		node.StartDaemon()

		t.Run("conns and streams are above 800 for default connmgr settings", func(t *testing.T) {
			res := node.RunIPFS("swarm", "resources", "--enc=json")
			require.Equal(t, 0, res.ExitCode())
			limits := unmarshalLimits(t, res.Stdout.Bytes())

			if limits.System.ConnsInbound != rcmgr.Unlimited {
				assert.GreaterOrEqual(t, limits.System.ConnsInbound, 800)
			}
			if limits.System.StreamsInbound != rcmgr.Unlimited {
				assert.GreaterOrEqual(t, limits.System.StreamsInbound, 800)
			}
		})

		t.Run("limits should succeed", func(t *testing.T) {
			res := node.RunIPFS("swarm", "resources", "--enc=json")
			assert.Equal(t, 0, res.ExitCode())

			limits := rcmgr.PartialLimitConfig{}
			err := json.Unmarshal(res.Stdout.Bytes(), &limits)
			require.NoError(t, err)

			assert.NotEqual(t, limits.Transient.Memory, rcmgr.BlockAllLimit64)
			assert.NotEqual(t, limits.System.Memory, rcmgr.BlockAllLimit64)
			assert.NotEqual(t, limits.System.FD, rcmgr.BlockAllLimit)
			assert.NotEqual(t, limits.System.Conns, rcmgr.BlockAllLimit)
			assert.NotEqual(t, limits.System.ConnsInbound, rcmgr.BlockAllLimit)
			assert.NotEqual(t, limits.System.ConnsOutbound, rcmgr.BlockAllLimit)
			assert.NotEqual(t, limits.System.Streams, rcmgr.BlockAllLimit)
			assert.NotEqual(t, limits.System.StreamsInbound, rcmgr.BlockAllLimit)
			assert.NotEqual(t, limits.System.StreamsOutbound, rcmgr.BlockAllLimit)
		})

		t.Run("swarm stats works", func(t *testing.T) {
			res := node.RunIPFS("swarm", "resources", "--enc=json")
			require.Equal(t, 0, res.ExitCode())

			limits := unmarshalLimits(t, res.Stdout.Bytes())

			// every scope has the same fields, so we only inspect system
			assert.Zero(t, limits.System.MemoryUsage)
			assert.Zero(t, limits.System.FDUsage)
			assert.Zero(t, limits.System.ConnsInboundUsage)
			assert.Zero(t, limits.System.ConnsOutboundUsage)
			assert.Zero(t, limits.System.StreamsInboundUsage)
			assert.Zero(t, limits.System.StreamsOutboundUsage)
			assert.Zero(t, limits.Transient.MemoryUsage)
		})
	})

	t.Run("smoke test transient scope", func(t *testing.T) {
		node := harness.NewT(t).NewNode().Init()
		node.UpdateUserSuppliedResourceManagerOverrides(func(overrides *rcmgr.PartialLimitConfig) {
			overrides.Transient.Memory = 88888
		})
		node.StartDaemon()

		res := node.RunIPFS("swarm", "resources", "--enc=json")
		limits := unmarshalLimits(t, res.Stdout.Bytes())
		assert.Equal(t, rcmgr.LimitVal64(88888), limits.Transient.Memory)
	})

	t.Run("smoke test service scope", func(t *testing.T) {
		node := harness.NewT(t).NewNode().Init()
		node.UpdateUserSuppliedResourceManagerOverrides(func(overrides *rcmgr.PartialLimitConfig) {
			overrides.Service = map[string]rcmgr.ResourceLimits{"foo": {Memory: 77777}}
		})
		node.StartDaemon()

		res := node.RunIPFS("swarm", "resources", "--enc=json")
		limits := unmarshalLimits(t, res.Stdout.Bytes())
		assert.Equal(t, rcmgr.LimitVal64(77777), limits.Services["foo"].Memory)
	})

	t.Run("smoke test protocol scope", func(t *testing.T) {
		node := harness.NewT(t).NewNode().Init()
		node.UpdateUserSuppliedResourceManagerOverrides(func(overrides *rcmgr.PartialLimitConfig) {
			overrides.Protocol = map[protocol.ID]rcmgr.ResourceLimits{"foo": {Memory: 66666}}
		})
		node.StartDaemon()

		res := node.RunIPFS("swarm", "resources", "--enc=json")
		limits := unmarshalLimits(t, res.Stdout.Bytes())
		assert.Equal(t, rcmgr.LimitVal64(66666), limits.Protocols["foo"].Memory)
	})

	t.Run("smoke test peer scope", func(t *testing.T) {
		validPeerID, err := peer.Decode("QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN")
		assert.NoError(t, err)
		node := harness.NewT(t).NewNode().Init()
		node.UpdateUserSuppliedResourceManagerOverrides(func(overrides *rcmgr.PartialLimitConfig) {
			overrides.Peer = map[peer.ID]rcmgr.ResourceLimits{validPeerID: {Memory: 55555}}
		})
		node.StartDaemon()

		res := node.RunIPFS("swarm", "resources", "--enc=json")
		limits := unmarshalLimits(t, res.Stdout.Bytes())
		assert.Equal(t, rcmgr.LimitVal64(55555), limits.Peers[validPeerID].Memory)
	})

	t.Run("", func(t *testing.T) {
		nodes := harness.NewT(t).NewNodes(3).Init()
		node0, node1, node2 := nodes[0], nodes[1], nodes[2]
		// peerID0, peerID1, peerID2 := node0.PeerID(), node1.PeerID(), node2.PeerID()
		peerID1, peerID2 := node1.PeerID().String(), node2.PeerID().String()

		node0.UpdateConfigAndUserSuppliedResourceManagerOverrides(func(cfg *config.Config, overrides *rcmgr.PartialLimitConfig) {
			*overrides = rcmgr.PartialLimitConfig{
				System: rcmgr.ResourceLimits{
					Conns:         rcmgr.BlockAllLimit,
					ConnsInbound:  rcmgr.BlockAllLimit,
					ConnsOutbound: rcmgr.BlockAllLimit,
				},
			}
			cfg.Swarm.ResourceMgr.Enabled = config.True
			cfg.Swarm.ResourceMgr.Allowlist = []string{"/ip4/0.0.0.0/ipcidr/0/p2p/" + peerID2}
		})

		nodes.StartDaemons()

		t.Parallel()
		t.Run("node 0 should fail to connect to node 1", func(t *testing.T) {
			res := node0.Runner.Run(harness.RunRequest{
				Path: node0.IPFSBin,
				Args: []string{"swarm", "connect", node1.SwarmAddrs()[0].String()},
			})
			assert.Equal(t, 1, res.ExitCode())
			assert.Contains(t, res.Stderr.String(), "failed to find any peer in table")
		})

		t.Run("node 0 should connect to node 2 since it is allowlisted", func(t *testing.T) {
			res := node0.Runner.Run(harness.RunRequest{
				Path: node0.IPFSBin,
				Args: []string{"swarm", "connect", node2.SwarmAddrs()[0].String()},
			})
			assert.Equal(t, 0, res.ExitCode())
		})

		t.Run("node 0 should fail to ping node 1", func(t *testing.T) {
			res := node0.RunIPFS("ping", "-n2", peerID1)
			assert.Equal(t, 1, res.ExitCode())
			assert.Contains(t, res.Stderr.String(), "Error: ping failed")
		})

		t.Run("node 0 should be able to ping node 2", func(t *testing.T) {
			res := node0.RunIPFS("ping", "-n2", peerID2)
			assert.Equal(t, 0, res.ExitCode())
		})
	})

	t.Run("daemon should refuse to start if connmgr.highwater < resources inbound", func(t *testing.T) {
		t.Parallel()
		t.Run("system conns", func(t *testing.T) {
			node := harness.NewT(t).NewNode().Init()
			node.UpdateConfigAndUserSuppliedResourceManagerOverrides(func(cfg *config.Config, overrides *rcmgr.PartialLimitConfig) {
				*overrides = rcmgr.PartialLimitConfig{
					System: rcmgr.ResourceLimits{Conns: 128},
				}
				cfg.Swarm.ConnMgr.HighWater = config.NewOptionalInteger(128)
				cfg.Swarm.ConnMgr.LowWater = config.NewOptionalInteger(64)
			})

			res := node.RunIPFS("daemon")
			assert.Equal(t, 1, res.ExitCode())
		})
		t.Run("system conns inbound", func(t *testing.T) {
			node := harness.NewT(t).NewNode().Init()
			node.UpdateConfigAndUserSuppliedResourceManagerOverrides(func(cfg *config.Config, overrides *rcmgr.PartialLimitConfig) {
				*overrides = rcmgr.PartialLimitConfig{
					System: rcmgr.ResourceLimits{ConnsInbound: 128},
				}
				cfg.Swarm.ConnMgr.HighWater = config.NewOptionalInteger(128)
				cfg.Swarm.ConnMgr.LowWater = config.NewOptionalInteger(64)
			})

			res := node.RunIPFS("daemon")
			assert.Equal(t, 1, res.ExitCode())
		})
		t.Run("system streams", func(t *testing.T) {
			node := harness.NewT(t).NewNode().Init()
			node.UpdateConfigAndUserSuppliedResourceManagerOverrides(func(cfg *config.Config, overrides *rcmgr.PartialLimitConfig) {
				*overrides = rcmgr.PartialLimitConfig{
					System: rcmgr.ResourceLimits{Streams: 128},
				}
				cfg.Swarm.ConnMgr.HighWater = config.NewOptionalInteger(128)
				cfg.Swarm.ConnMgr.LowWater = config.NewOptionalInteger(64)
			})

			res := node.RunIPFS("daemon")
			assert.Equal(t, 1, res.ExitCode())
		})
		t.Run("system streams inbound", func(t *testing.T) {
			node := harness.NewT(t).NewNode().Init()
			node.UpdateConfigAndUserSuppliedResourceManagerOverrides(func(cfg *config.Config, overrides *rcmgr.PartialLimitConfig) {
				*overrides = rcmgr.PartialLimitConfig{
					System: rcmgr.ResourceLimits{StreamsInbound: 128},
				}
				cfg.Swarm.ConnMgr.HighWater = config.NewOptionalInteger(128)
				cfg.Swarm.ConnMgr.LowWater = config.NewOptionalInteger(64)
			})

			res := node.RunIPFS("daemon")
			assert.Equal(t, 1, res.ExitCode())
		})
	})
}

func unmarshalLimits(t *testing.T, b []byte) *libp2p.LimitsConfigAndUsage {
	limits := &libp2p.LimitsConfigAndUsage{}
	err := json.Unmarshal(b, limits)
	require.NoError(t, err)
	return limits
}
