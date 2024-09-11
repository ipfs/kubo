package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/ipfs/kubo/test/cli/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransports(t *testing.T) {
	disableRouting := func(nodes harness.Nodes) {
		nodes.ForEachPar(func(n *harness.Node) {
			n.UpdateConfig(func(cfg *config.Config) {
				cfg.Routing.Type = config.NewOptionalString("none")
				cfg.Bootstrap = nil
			})
		})
	}
	checkSingleFile := func(nodes harness.Nodes) {
		s := testutils.RandomStr(100)
		hash := nodes[0].IPFSAddStr(s)
		nodes.ForEachPar(func(n *harness.Node) {
			val := n.IPFS("cat", hash).Stdout.String()
			assert.Equal(t, s, val)
		})
	}
	checkRandomDir := func(nodes harness.Nodes) {
		randDir := filepath.Join(nodes[0].Dir, "foobar")
		require.NoError(t, os.Mkdir(randDir, 0o777))
		rf := testutils.NewRandFiles()
		rf.FanoutDirs = 3
		rf.FanoutFiles = 6
		require.NoError(t, rf.WriteRandomFiles(randDir, 4))

		hash := nodes[1].IPFS("add", "-r", "-Q", randDir).Stdout.Trimmed()
		nodes.ForEachPar(func(n *harness.Node) {
			res := n.RunIPFS("refs", "-r", hash)
			assert.Equal(t, 0, res.ExitCode())
		})
	}

	runTests := func(nodes harness.Nodes) {
		checkSingleFile(nodes)
		checkRandomDir(nodes)
	}

	tcpNodes := func(t *testing.T) harness.Nodes {
		nodes := harness.NewT(t).NewNodes(2).Init()
		nodes.ForEachPar(func(n *harness.Node) {
			n.UpdateConfig(func(cfg *config.Config) {
				cfg.Addresses.Swarm = []string{"/ip4/127.0.0.1/tcp/0"}
				cfg.Swarm.Transports.Network.QUIC = config.False
				cfg.Swarm.Transports.Network.Relay = config.False
				cfg.Swarm.Transports.Network.WebTransport = config.False
				cfg.Swarm.Transports.Network.WebRTCDirect = config.False
				cfg.Swarm.Transports.Network.Websocket = config.False
			})
		})
		disableRouting(nodes)
		return nodes
	}

	t.Run("tcp", func(t *testing.T) {
		t.Parallel()
		nodes := tcpNodes(t).StartDaemons().Connect()
		runTests(nodes)
	})

	t.Run("tcp with NOISE", func(t *testing.T) {
		t.Parallel()
		nodes := tcpNodes(t)
		nodes.ForEachPar(func(n *harness.Node) {
			n.UpdateConfig(func(cfg *config.Config) {
				cfg.Swarm.Transports.Security.TLS = config.Disabled
			})
		})
		nodes.StartDaemons().Connect()
		runTests(nodes)
	})

	t.Run("QUIC", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(5).Init()
		nodes.ForEachPar(func(n *harness.Node) {
			n.UpdateConfig(func(cfg *config.Config) {
				cfg.Addresses.Swarm = []string{"/ip4/127.0.0.1/udp/0/quic-v1"}
				cfg.Swarm.Transports.Network.TCP = config.False
				cfg.Swarm.Transports.Network.QUIC = config.True
				cfg.Swarm.Transports.Network.WebTransport = config.False
				cfg.Swarm.Transports.Network.WebRTCDirect = config.False
			})
		})
		disableRouting(nodes)
		nodes.StartDaemons().Connect()
		runTests(nodes)
	})

	t.Run("QUIC+Webtransport", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(5).Init()
		nodes.ForEachPar(func(n *harness.Node) {
			n.UpdateConfig(func(cfg *config.Config) {
				cfg.Addresses.Swarm = []string{"/ip4/127.0.0.1/udp/0/quic-v1/webtransport"}
				cfg.Swarm.Transports.Network.TCP = config.False
				cfg.Swarm.Transports.Network.QUIC = config.True
				cfg.Swarm.Transports.Network.WebTransport = config.True
				cfg.Swarm.Transports.Network.WebRTCDirect = config.False
			})
		})
		disableRouting(nodes)
		nodes.StartDaemons().Connect()
		runTests(nodes)
	})

	t.Run("QUIC connects with non-dialable transports", func(t *testing.T) {
		// This test targets specific Kubo internals which may change later. This checks
		// if we can announce an address we do not listen on, and then are able to connect
		// via a different address that is available.
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(5).Init()
		nodes.ForEachPar(func(n *harness.Node) {
			n.UpdateConfig(func(cfg *config.Config) {
				// We need a specific port to announce so we first generate a random port.
				// We can't use 0 here to automatically assign an available port because
				// that would only work with Swarm, but not for the announcing.
				port := harness.NewRandPort()
				quicAddr := fmt.Sprintf("/ip4/127.0.0.1/udp/%d/quic-v1", port)
				cfg.Addresses.Swarm = []string{quicAddr}
				cfg.Addresses.Announce = []string{quicAddr, quicAddr + "/webtransport"}
			})
		})
		disableRouting(nodes)
		nodes.StartDaemons().Connect()
		runTests(nodes)
	})

	t.Run("WebRTC Direct", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(5).Init()
		nodes.ForEachPar(func(n *harness.Node) {
			n.UpdateConfig(func(cfg *config.Config) {
				cfg.Addresses.Swarm = []string{"/ip4/127.0.0.1/udp/0/webrtc-direct"}
				cfg.Swarm.Transports.Network.TCP = config.False
				cfg.Swarm.Transports.Network.QUIC = config.False
				cfg.Swarm.Transports.Network.WebTransport = config.False
				cfg.Swarm.Transports.Network.WebRTCDirect = config.True
			})
		})
		disableRouting(nodes)
		nodes.StartDaemons().Connect()
		runTests(nodes)
	})
}
