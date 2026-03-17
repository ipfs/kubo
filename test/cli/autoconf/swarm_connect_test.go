package autoconf

import (
	"testing"
	"time"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSwarmConnectWithAutoConf tests that ipfs swarm connect works properly
// when AutoConf is enabled and a daemon is running.
//
// This is a regression test for the issue where:
// - AutoConf disabled: ipfs swarm connect works
// - AutoConf enabled: ipfs swarm connect fails with "Error: connect"
//
// The issue affects CLI command fallback behavior when the HTTP API connection fails.
func TestSwarmConnectWithAutoConf(t *testing.T) {
	t.Parallel()

	t.Run("AutoConf disabled - should work", func(t *testing.T) {
		testSwarmConnectWithAutoConfSetting(t, false, true) // expect success
	})

	t.Run("AutoConf enabled - should work", func(t *testing.T) {
		testSwarmConnectWithAutoConfSetting(t, true, true) // expect success (fix the bug!)
	})
}

func testSwarmConnectWithAutoConfSetting(t *testing.T, autoConfEnabled bool, expectSuccess bool) {
	// Create IPFS node with test profile
	node := harness.NewT(t).NewNode().Init("--profile=test")

	// Configure AutoConf
	node.SetIPFSConfig("AutoConf.Enabled", autoConfEnabled)

	// Set up bootstrap peers so the node has something to connect to
	// Use the same bootstrap peers from boxo/autoconf fallbacks
	node.SetIPFSConfig("Bootstrap", []string{
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
	})

	// CRITICAL: Start the daemon first - this is the key requirement
	// The daemon must be running and working properly
	node.StartDaemon()
	defer node.StopDaemon()

	// Give daemon time to start up completely
	time.Sleep(3 * time.Second)

	// Verify daemon is responsive
	result := node.RunIPFS("id")
	require.Equal(t, 0, result.ExitCode(), "Daemon should be responsive before testing swarm connect")
	t.Logf("Daemon is running and responsive. AutoConf enabled: %v", autoConfEnabled)

	// Now test swarm connect to a bootstrap peer
	// This should work because:
	// 1. The daemon is running
	// 2. The CLI should connect to the daemon via API
	// 3. The daemon should handle the swarm connect request
	result = node.RunIPFS("swarm", "connect", "/dnsaddr/bootstrap.libp2p.io")

	// swarm connect should work regardless of AutoConf setting
	assert.Equal(t, 0, result.ExitCode(),
		"swarm connect should succeed with AutoConf=%v. stderr: %s",
		autoConfEnabled, result.Stderr.String())

	// Should contain success message
	output := result.Stdout.String()
	assert.Contains(t, output, "success",
		"swarm connect output should contain 'success' with AutoConf=%v. output: %s",
		autoConfEnabled, output)

	// Additional diagnostic: Check if ipfs id shows addresses
	// Both AutoConf enabled and disabled should show proper addresses
	result = node.RunIPFS("id")
	require.Equal(t, 0, result.ExitCode(), "ipfs id should work with AutoConf=%v", autoConfEnabled)

	idOutput := result.Stdout.String()
	t.Logf("ipfs id output with AutoConf=%v: %s", autoConfEnabled, idOutput)

	// Addresses should not be null regardless of AutoConf setting
	assert.Contains(t, idOutput, `"Addresses"`, "ipfs id should show Addresses field")
	assert.NotContains(t, idOutput, `"Addresses": null`,
		"ipfs id should not show null addresses with AutoConf=%v", autoConfEnabled)
}
