package cli

import (
	"encoding/json"
	"testing"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
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

		t.Run("swarm limit should fail", func(t *testing.T) {
			res := node.RunIPFS("swarm", "limit", "system")
			assert.Equal(t, 1, res.ExitCode())
			assert.Contains(t, res.Stderr.Lines()[0], "missing ResourceMgr")
		})
		t.Run("swarm stats should fail", func(t *testing.T) {
			res := node.RunIPFS("swarm", "stats", "all")
			assert.Equal(t, 1, res.ExitCode())
			assert.Contains(t, res.Stderr.Lines()[0], "missing ResourceMgr")
		})
	})

	t.Run("Node in offline mode", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Swarm.ResourceMgr.Enabled = config.False
		})
		node.StartDaemon()

		t.Run("swarm limit should fail", func(t *testing.T) {
			res := node.RunIPFS("swarm", "limit", "system")
			assert.Equal(t, 1, res.ExitCode())
			assert.Contains(t, res.Stderr.Lines()[0], "missing ResourceMgr")
		})
		t.Run("swarm stats should fail", func(t *testing.T) {
			res := node.RunIPFS("swarm", "stats", "all")
			assert.Equal(t, 1, res.ExitCode())
			assert.Contains(t, res.Stderr.Lines()[0], "missing ResourceMgr")
		})
	})

	t.Run("Very high connmgr highwater", func(t *testing.T) {
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Swarm.ConnMgr.HighWater = config.NewOptionalInteger(1000)
		})
		node.StartDaemon()

		res := node.RunIPFS("swarm", "limit", "system", "--enc=json")
		require.Equal(t, 0, res.ExitCode())
		limits := unmarshalLimits(t, res.Stdout.Bytes())

		assert.GreaterOrEqual(t, limits.ConnsInbound, 2000)
		assert.GreaterOrEqual(t, limits.StreamsInbound, 2000)
	})

	t.Run("default configuration", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Swarm.ConnMgr.HighWater = config.NewOptionalInteger(1000)
		})
		node.StartDaemon()

		t.Run("conns and streams are above 800 for default connmgr settings", func(t *testing.T) {
			res := node.RunIPFS("swarm", "limit", "system", "--enc=json")
			require.Equal(t, 0, res.ExitCode())
			limits := unmarshalLimits(t, res.Stdout.Bytes())

			assert.GreaterOrEqual(t, limits.ConnsInbound, 800)
			assert.GreaterOrEqual(t, limits.StreamsInbound, 800)
		})

		t.Run("limits|stats should succeed", func(t *testing.T) {
			res := node.RunIPFS("swarm", "limit", "all")
			assert.Equal(t, 0, res.ExitCode())

			limits := map[string]rcmgr.ResourceLimits{}
			err := json.Unmarshal(res.Stdout.Bytes(), &limits)
			require.NoError(t, err)

			assert.Greater(t, limits["System"].Memory, int64(0))
			assert.Greater(t, limits["System"].FD, 0)
			assert.Greater(t, limits["System"].Conns, 0)
			assert.Greater(t, limits["System"].ConnsInbound, 0)
			assert.Greater(t, limits["System"].ConnsOutbound, 0)
			assert.Greater(t, limits["System"].Streams, 0)
			assert.Greater(t, limits["System"].StreamsInbound, 0)
			assert.Greater(t, limits["System"].StreamsOutbound, 0)
			assert.Greater(t, limits["Transient"].Memory, int64(0))
		})

		t.Run("resetting limits should produce the same default limits", func(t *testing.T) {
			resetRes := node.RunIPFS("swarm", "limit", "system", "--reset", "--enc=json")
			require.Equal(t, 0, resetRes.ExitCode())
			limitRes := node.RunIPFS("swarm", "limit", "system", "--enc=json")
			require.Equal(t, 0, limitRes.ExitCode())

			assert.Equal(t, resetRes.Stdout.Bytes(), limitRes.Stdout.Bytes())
		})

		t.Run("swarm stats system with filter should fail", func(t *testing.T) {
			res := node.RunIPFS("swarm", "stats", "system", "--min-used-limit-perc=99")
			assert.Equal(t, 1, res.ExitCode())
			assert.Contains(t, res.Stderr.Lines()[0], `Error: "min-used-limit-perc" can only be used when scope is "all"`)
		})

		t.Run("swarm limit reset on map values should work", func(t *testing.T) {
			resetRes := node.RunIPFS("swarm", "limit", "peer:12D3KooWL7i1T9VSPeF8AgQApbyM51GNKZsYPvNvL347aMDmvNzG", "--reset", "--enc=json")
			require.Equal(t, 0, resetRes.ExitCode())
			limitRes := node.RunIPFS("swarm", "limit", "peer:12D3KooWL7i1T9VSPeF8AgQApbyM51GNKZsYPvNvL347aMDmvNzG", "--enc=json")
			require.Equal(t, 0, limitRes.ExitCode())

			assert.Equal(t, resetRes.Stdout.Bytes(), limitRes.Stdout.Bytes())
		})

		t.Run("scope is required using reset flags", func(t *testing.T) {
			res := node.RunIPFS("swarm", "limit", "--reset")
			assert.Equal(t, 1, res.ExitCode())
			assert.Contains(t, res.Stderr.Lines()[0], `Error: argument "scope" is required`)
		})

		t.Run("swarm stats works", func(t *testing.T) {
			res := node.RunIPFS("swarm", "stats", "all", "--enc=json")
			require.Equal(t, 0, res.ExitCode())

			stats := rcmgr.PartialLimitConfig{}
			err := json.Unmarshal(res.Stdout.Bytes(), &stats)
			require.NoError(t, err)

			// every scope has the same fields, so we only inspect system
			assert.Equal(t, rcmgr.LimitVal64(0), stats.System.Memory)
			assert.Equal(t, rcmgr.LimitVal(0), stats.System.FD)
			assert.Equal(t, rcmgr.LimitVal(0), stats.System.Conns)
			assert.Equal(t, rcmgr.LimitVal(0), stats.System.ConnsInbound)
			assert.Equal(t, rcmgr.LimitVal(0), stats.System.ConnsOutbound)
			assert.Equal(t, rcmgr.LimitVal(0), stats.System.Streams)
			assert.Equal(t, rcmgr.LimitVal(0), stats.System.StreamsInbound)
			assert.Equal(t, rcmgr.LimitVal(0), stats.System.StreamsOutbound)
			assert.Equal(t, rcmgr.LimitVal64(0), stats.Transient.Memory)
		})
	})

	t.Run("set system conns limit while daemon is not running", func(t *testing.T) {
		node := harness.NewT(t).NewNode().Init()
		res := node.RunIPFS("config", "--json", "Swarm.ResourceMgr.Limits.System.Conns", "99999")
		require.Equal(t, 0, res.ExitCode())

		t.Run("set an invalid limit which should result in a failure", func(t *testing.T) {
			res := node.RunIPFS("config", "--json", "Swarm.ResourceMgr.Limits.System.Conns", "asdf")
			assert.Equal(t, 1, res.ExitCode())
			assert.Contains(t, res.Stderr.String(), "failed to unmarshal")
		})

		node.StartDaemon()

		t.Run("new system conns limit is applied", func(t *testing.T) {
			res := node.RunIPFS("swarm", "limit", "system", "--enc=json")
			limits := unmarshalLimits(t, res.Stdout.Bytes())
			assert.Equal(t, limits.Conns, rcmgr.LimitVal(99999))
		})
	})

	t.Run("set the system memory limit while the daemon is running", func(t *testing.T) {
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		updateLimitsWithFile(t, node, "system", func(limits *rcmgr.ResourceLimits) {
			limits.Memory = 99998
		})

		assert.Equal(t, rcmgr.LimitVal64(99998), node.ReadConfig().Swarm.ResourceMgr.Limits.System.Memory)

		res := node.RunIPFS("swarm", "limit", "system", "--enc=json")
		limits := unmarshalLimits(t, res.Stdout.Bytes())
		assert.Equal(t, rcmgr.LimitVal64(99998), limits.Memory)
	})

	t.Run("smoke test transient scope", func(t *testing.T) {
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		updateLimitsWithFile(t, node, "transient", func(limits *rcmgr.ResourceLimits) {
			limits.Memory = 88888
		})

		res := node.RunIPFS("swarm", "limit", "transient", "--enc=json")
		limits := unmarshalLimits(t, res.Stdout.Bytes())
		assert.Equal(t, rcmgr.LimitVal64(88888), limits.Memory)
	})

	t.Run("smoke test service scope", func(t *testing.T) {
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		updateLimitsWithFile(t, node, "svc:foo", func(limits *rcmgr.ResourceLimits) {
			limits.Memory = 77777
		})

		res := node.RunIPFS("swarm", "limit", "svc:foo", "--enc=json")
		limits := unmarshalLimits(t, res.Stdout.Bytes())
		assert.Equal(t, rcmgr.LimitVal64(77777), limits.Memory)
	})

	t.Run("smoke test protocol scope", func(t *testing.T) {
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		updateLimitsWithFile(t, node, "proto:foo", func(limits *rcmgr.ResourceLimits) {
			limits.Memory = 66666
		})

		res := node.RunIPFS("swarm", "limit", "proto:foo", "--enc=json")
		limits := unmarshalLimits(t, res.Stdout.Bytes())
		assert.Equal(t, rcmgr.LimitVal64(66666), limits.Memory)
	})

	t.Run("smoke test peer scope", func(t *testing.T) {
		validPeerID := "QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN"
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		updateLimitsWithFile(t, node, "peer:"+validPeerID, func(limits *rcmgr.ResourceLimits) {
			limits.Memory = 66666
		})

		res := node.RunIPFS("swarm", "limit", "peer:"+validPeerID, "--enc=json")
		limits := unmarshalLimits(t, res.Stdout.Bytes())
		assert.Equal(t, rcmgr.LimitVal64(66666), limits.Memory)

		t.Parallel()

		t.Run("getting limit for invalid peer ID fails", func(t *testing.T) {
			res := node.RunIPFS("swarm", "limit", "peer:foo")
			assert.Equal(t, 1, res.ExitCode())
			assert.Contains(t, res.Stderr.String(), "invalid peer ID")
		})

		t.Run("setting limit for invalid peer ID fails", func(t *testing.T) {
			filename := "invalid-peer-id.json"
			node.WriteBytes(filename, []byte(`{"Memory":"99"}`))
			res := node.RunIPFS("swarm", "limit", "peer:foo", filename)
			assert.Equal(t, 1, res.ExitCode())
			assert.Contains(t, res.Stderr.String(), "invalid peer ID")
		})
	})

	t.Run("", func(t *testing.T) {
		nodes := harness.NewT(t).NewNodes(3).Init()
		node0, node1, node2 := nodes[0], nodes[1], nodes[2]
		// peerID0, peerID1, peerID2 := node0.PeerID(), node1.PeerID(), node2.PeerID()
		peerID1, peerID2 := node1.PeerID().String(), node2.PeerID().String()

		node0.UpdateConfig(func(cfg *config.Config) {
			cfg.Swarm.ResourceMgr.Enabled = config.True
			cfg.Swarm.ResourceMgr.Allowlist = []string{"/ip4/0.0.0.0/ipcidr/0/p2p/" + peerID2}
		})

		nodes.StartDaemons()

		// change system limits on node 0
		updateLimitsWithFile(t, node0, "system", func(limits *rcmgr.ResourceLimits) {
			limits.Conns = rcmgr.BlockAllLimit
			limits.ConnsInbound = rcmgr.BlockAllLimit
			limits.ConnsOutbound = rcmgr.BlockAllLimit
		})

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
			node.UpdateConfig(func(cfg *config.Config) {
				cfg.Swarm.ResourceMgr.Limits = &rcmgr.PartialLimitConfig{}
				cfg.Swarm.ResourceMgr.Limits.System.Conns = 128
				cfg.Swarm.ConnMgr.HighWater = config.NewOptionalInteger(128)
				cfg.Swarm.ConnMgr.LowWater = config.NewOptionalInteger(64)
			})

			res := node.RunIPFS("daemon")
			assert.Equal(t, 1, res.ExitCode())
		})
		t.Run("system conns inbound", func(t *testing.T) {
			node := harness.NewT(t).NewNode().Init()
			node.UpdateConfig(func(cfg *config.Config) {
				cfg.Swarm.ResourceMgr.Limits = &rcmgr.PartialLimitConfig{}
				cfg.Swarm.ResourceMgr.Limits.System.ConnsInbound = 128
				cfg.Swarm.ConnMgr.HighWater = config.NewOptionalInteger(128)
				cfg.Swarm.ConnMgr.LowWater = config.NewOptionalInteger(64)
			})

			res := node.RunIPFS("daemon")
			assert.Equal(t, 1, res.ExitCode())
		})
		t.Run("system streams", func(t *testing.T) {
			node := harness.NewT(t).NewNode().Init()
			node.UpdateConfig(func(cfg *config.Config) {
				cfg.Swarm.ResourceMgr.Limits = &rcmgr.PartialLimitConfig{}
				cfg.Swarm.ResourceMgr.Limits.System.Streams = 128
				cfg.Swarm.ConnMgr.HighWater = config.NewOptionalInteger(128)
				cfg.Swarm.ConnMgr.LowWater = config.NewOptionalInteger(64)
			})

			res := node.RunIPFS("daemon")
			assert.Equal(t, 1, res.ExitCode())
		})
		t.Run("system streams inbound", func(t *testing.T) {
			node := harness.NewT(t).NewNode().Init()
			node.UpdateConfig(func(cfg *config.Config) {
				cfg.Swarm.ResourceMgr.Limits = &rcmgr.PartialLimitConfig{}
				cfg.Swarm.ResourceMgr.Limits.System.StreamsInbound = 128
				cfg.Swarm.ConnMgr.HighWater = config.NewOptionalInteger(128)
				cfg.Swarm.ConnMgr.LowWater = config.NewOptionalInteger(64)
			})

			res := node.RunIPFS("daemon")
			assert.Equal(t, 1, res.ExitCode())
		})
	})
}

func updateLimitsWithFile(t *testing.T, node *harness.Node, limit string, f func(*rcmgr.ResourceLimits)) {
	filename := limit + ".json"
	res := node.RunIPFS("swarm", "limit", limit)
	limits := unmarshalLimits(t, res.Stdout.Bytes())

	f(limits)

	limitsOut, err := json.Marshal(limits)
	require.NoError(t, err)
	node.WriteBytes(filename, limitsOut)
	res = node.RunIPFS("swarm", "limit", limit, filename)
	assert.Equal(t, 0, res.ExitCode())
}

func unmarshalLimits(t *testing.T, b []byte) *rcmgr.ResourceLimits {
	limits := &rcmgr.ResourceLimits{}
	err := json.Unmarshal(b, limits)
	require.NoError(t, err)
	return limits
}
