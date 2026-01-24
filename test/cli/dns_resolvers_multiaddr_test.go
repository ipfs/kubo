package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testDomainSuffix is the default p2p-forge domain used in tests
const testDomainSuffix = config.DefaultDomainSuffix // libp2p.direct

// TestDNSResolversApplyToMultiaddr is a regression test for:
// https://github.com/ipfs/kubo/issues/9199
//
// It verifies that DNS.Resolvers config is used when resolving /dnsaddr,
// /dns, /dns4, /dns6 multiaddrs during peer connections, not just for
// DNSLink resolution.
func TestDNSResolversApplyToMultiaddr(t *testing.T) {
	t.Parallel()

	t.Run("invalid DoH resolver causes multiaddr resolution to fail", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init("--profile=test")

		// Set an invalid DoH resolver that will fail when used.
		// If DNS.Resolvers is properly wired to multiaddr resolution,
		// swarm connect to a /dnsaddr will fail with an error mentioning
		// the invalid resolver URL.
		invalidResolver := "https://invalid.broken.resolver.test/dns-query"
		node.SetIPFSConfig("DNS.Resolvers", map[string]string{
			".": invalidResolver,
		})

		// Clear bootstrap peers to prevent background connection attempts
		node.SetIPFSConfig("Bootstrap", []string{})

		node.StartDaemon()
		defer node.StopDaemon()

		// Give daemon time to fully start
		time.Sleep(2 * time.Second)

		// Verify daemon is responsive
		result := node.RunIPFS("id")
		require.Equal(t, 0, result.ExitCode(), "daemon should be responsive")

		// Try to connect to a /dnsaddr peer - this should fail because
		// the DNS.Resolvers config points to an invalid DoH server
		result = node.RunIPFS("swarm", "connect", "/dnsaddr/bootstrap.libp2p.io")

		// The connection should fail
		require.NotEqual(t, 0, result.ExitCode(),
			"swarm connect should fail when DNS.Resolvers points to invalid DoH server")

		// The error should mention the invalid resolver, proving DNS.Resolvers
		// is being used for multiaddr resolution
		stderr := result.Stderr.String()
		assert.True(t,
			strings.Contains(stderr, "invalid.broken.resolver.test") ||
				strings.Contains(stderr, "no such host") ||
				strings.Contains(stderr, "lookup") ||
				strings.Contains(stderr, "dial"),
			"error should indicate DNS resolution failure using custom resolver. got: %s", stderr)
	})

	t.Run("libp2p.direct resolves locally even with broken DNS.Resolvers", func(t *testing.T) {
		t.Parallel()

		h := harness.NewT(t)
		nodes := h.NewNodes(2).Init("--profile=test")

		// Configure node0 with a broken DNS resolver
		// This would break all DNS resolution if libp2p.direct wasn't resolved locally
		invalidResolver := "https://invalid.broken.resolver.test/dns-query"
		nodes[0].SetIPFSConfig("DNS.Resolvers", map[string]string{
			".": invalidResolver,
		})

		// Clear bootstrap peers on both nodes
		for _, n := range nodes {
			n.SetIPFSConfig("Bootstrap", []string{})
		}

		nodes.StartDaemons()
		defer nodes.StopDaemons()

		// Get node1's peer ID in base36 format (what p2p-forge uses in DNS hostnames)
		// DNS is case-insensitive, and base36 is lowercase-only, making it ideal for DNS
		idResult := nodes[1].RunIPFS("id", "--peerid-base", "base36", "-f", "<id>")
		require.Equal(t, 0, idResult.ExitCode())
		node1IDBase36 := strings.TrimSpace(idResult.Stdout.String())
		node1ID := nodes[1].PeerID().String()
		node1Addrs := nodes[1].SwarmAddrs()

		// Find a TCP address we can use
		var tcpAddr string
		for _, addr := range node1Addrs {
			addrStr := addr.String()
			if strings.Contains(addrStr, "/tcp/") && strings.Contains(addrStr, "/ip4/127.0.0.1") {
				tcpAddr = addrStr
				break
			}
		}
		require.NotEmpty(t, tcpAddr, "node1 should have a local TCP address")

		// Extract port from address like /ip4/127.0.0.1/tcp/12345/...
		parts := strings.Split(tcpAddr, "/")
		var port string
		for i, p := range parts {
			if p == "tcp" && i+1 < len(parts) {
				port = parts[i+1]
				break
			}
		}
		require.NotEmpty(t, port, "should find TCP port in address")

		// Construct a libp2p.direct hostname that encodes 127.0.0.1
		// Format: /dns4/<ip-encoded>.<peerID-base36>.libp2p.direct/tcp/<port>/p2p/<peerID>
		// p2p-forge uses base36 peerIDs in DNS hostnames (lowercase, DNS-safe)
		libp2pDirectAddr := "/dns4/127-0-0-1." + node1IDBase36 + "." + testDomainSuffix + "/tcp/" + port + "/p2p/" + node1ID

		// This connection should succeed because libp2p.direct is resolved locally
		// even though DNS.Resolvers points to a broken server
		result := nodes[0].RunIPFS("swarm", "connect", libp2pDirectAddr)

		// The connection should succeed - local resolution bypasses broken DNS
		assert.Equal(t, 0, result.ExitCode(),
			"swarm connect to libp2p.direct should succeed with local resolution. stderr: %s",
			result.Stderr.String())

		// Verify the connection was actually established
		result = nodes[0].RunIPFS("swarm", "peers")
		require.Equal(t, 0, result.ExitCode())
		assert.Contains(t, result.Stdout.String(), node1ID,
			"node0 should be connected to node1")
	})
}
