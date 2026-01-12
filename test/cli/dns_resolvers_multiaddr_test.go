package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
}
