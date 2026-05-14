package cli

import (
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBootstrapCommandsWithAutoPlaceholder(t *testing.T) {
	t.Parallel()

	t.Run("bootstrap add default", func(t *testing.T) {
		t.Parallel()
		// Test that 'ipfs bootstrap add default' works correctly
		node := harness.NewT(t).NewNode().Init("--profile=test")
		node.SetIPFSConfig("AutoConf.Enabled", true)
		node.SetIPFSConfig("Bootstrap", []string{}) // Start with empty bootstrap

		// Add default bootstrap peers via "auto" placeholder
		result := node.RunIPFS("bootstrap", "add", "default")
		require.Equal(t, 0, result.ExitCode(), "bootstrap add default should succeed")

		output := result.Stdout.String()
		t.Logf("Bootstrap add default output: %s", output)
		assert.Contains(t, output, "added auto", "bootstrap add default should report adding 'auto'")

		// Verify bootstrap list shows "auto"
		listResult := node.RunIPFS("bootstrap", "list")
		require.Equal(t, 0, listResult.ExitCode(), "bootstrap list should succeed")

		listOutput := listResult.Stdout.String()
		t.Logf("Bootstrap list after add default: %s", listOutput)
		assert.Contains(t, listOutput, "auto", "bootstrap list should show 'auto' placeholder")
	})

	t.Run("bootstrap add auto explicitly", func(t *testing.T) {
		t.Parallel()
		// Test that 'ipfs bootstrap add auto' works correctly
		node := harness.NewT(t).NewNode().Init("--profile=test")
		node.SetIPFSConfig("AutoConf.Enabled", true)
		node.SetIPFSConfig("Bootstrap", []string{}) // Start with empty bootstrap

		// Add "auto" placeholder explicitly
		result := node.RunIPFS("bootstrap", "add", "auto")
		require.Equal(t, 0, result.ExitCode(), "bootstrap add auto should succeed")

		output := result.Stdout.String()
		t.Logf("Bootstrap add auto output: %s", output)
		assert.Contains(t, output, "added auto", "bootstrap add auto should report adding 'auto'")

		// Verify bootstrap list shows "auto"
		listResult := node.RunIPFS("bootstrap", "list")
		require.Equal(t, 0, listResult.ExitCode(), "bootstrap list should succeed")

		listOutput := listResult.Stdout.String()
		t.Logf("Bootstrap list after add auto: %s", listOutput)
		assert.Contains(t, listOutput, "auto", "bootstrap list should show 'auto' placeholder")
	})

	t.Run("bootstrap add default converts to auto", func(t *testing.T) {
		t.Parallel()
		// Test that 'ipfs bootstrap add default' adds "auto" to the bootstrap list
		node := harness.NewT(t).NewNode().Init("--profile=test")
		node.SetIPFSConfig("Bootstrap", []string{})  // Start with empty bootstrap
		node.SetIPFSConfig("AutoConf.Enabled", true) // Enable AutoConf to allow adding "auto"

		// Add default bootstrap peers
		result := node.RunIPFS("bootstrap", "add", "default")
		require.Equal(t, 0, result.ExitCode(), "bootstrap add default should succeed")
		assert.Contains(t, result.Stdout.String(), "added auto", "should report adding 'auto'")

		// Verify bootstrap list shows "auto"
		var bootstrap []string
		node.GetIPFSConfig("Bootstrap", &bootstrap)
		require.Equal(t, []string{"auto"}, bootstrap, "Bootstrap should contain ['auto']")
	})

	t.Run("bootstrap add default fails when AutoConf disabled", func(t *testing.T) {
		t.Parallel()
		// Test that adding default/auto fails when AutoConf is disabled
		node := harness.NewT(t).NewNode().Init("--profile=test")
		node.SetIPFSConfig("Bootstrap", []string{})   // Start with empty bootstrap
		node.SetIPFSConfig("AutoConf.Enabled", false) // Disable AutoConf

		// Try to add default - should fail
		result := node.RunIPFS("bootstrap", "add", "default")
		require.NotEqual(t, 0, result.ExitCode(), "bootstrap add default should fail when AutoConf disabled")
		assert.Contains(t, result.Stderr.String(), "AutoConf is disabled", "should mention AutoConf is disabled")

		// Try to add auto - should also fail
		result = node.RunIPFS("bootstrap", "add", "auto")
		require.NotEqual(t, 0, result.ExitCode(), "bootstrap add auto should fail when AutoConf disabled")
		assert.Contains(t, result.Stderr.String(), "AutoConf is disabled", "should mention AutoConf is disabled")
	})

	t.Run("bootstrap rm with auto placeholder", func(t *testing.T) {
		t.Parallel()
		// Test that selective removal fails properly when "auto" is present
		node := harness.NewT(t).NewNode().Init("--profile=test")
		node.SetIPFSConfig("AutoConf.Enabled", true)
		node.SetIPFSConfig("Bootstrap", []string{"auto"}) // Start with auto

		// Try to remove a specific peer - should fail with helpful error
		result := node.RunIPFS("bootstrap", "rm", "/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN")
		require.NotEqual(t, 0, result.ExitCode(), "bootstrap rm of specific peer should fail when 'auto' is present")

		output := result.Stderr.String()
		t.Logf("Bootstrap rm error output: %s", output)
		assert.Contains(t, output, "cannot remove individual bootstrap peers when using 'auto' placeholder",
			"should provide helpful error message about auto placeholder")
		assert.Contains(t, output, "disable AutoConf",
			"should suggest disabling AutoConf as solution")
		assert.Contains(t, output, "ipfs bootstrap rm --all",
			"should suggest using rm --all as alternative")
	})

	t.Run("bootstrap rm --all with auto placeholder", func(t *testing.T) {
		t.Parallel()
		// Test that 'ipfs bootstrap rm --all' works with "auto" placeholder
		node := harness.NewT(t).NewNode().Init("--profile=test")
		node.SetIPFSConfig("AutoConf.Enabled", true)
		node.SetIPFSConfig("Bootstrap", []string{"auto"}) // Start with auto

		// Remove all bootstrap peers
		result := node.RunIPFS("bootstrap", "rm", "--all")
		require.Equal(t, 0, result.ExitCode(), "bootstrap rm --all should succeed with auto placeholder")

		output := result.Stdout.String()
		t.Logf("Bootstrap rm --all output: %s", output)
		assert.Contains(t, output, "removed auto", "bootstrap rm --all should report removing 'auto'")

		// Verify bootstrap list is now empty
		listResult := node.RunIPFS("bootstrap", "list")
		require.Equal(t, 0, listResult.ExitCode(), "bootstrap list should succeed")

		listOutput := listResult.Stdout.String()
		t.Logf("Bootstrap list after rm --all: %s", listOutput)
		assert.Empty(t, listOutput, "bootstrap list should be empty after rm --all")

		// Test the rm all subcommand too
		node.SetIPFSConfig("Bootstrap", []string{"auto"}) // Reset to auto

		result = node.RunIPFS("bootstrap", "rm", "all")
		require.Equal(t, 0, result.ExitCode(), "bootstrap rm all should succeed with auto placeholder")

		output = result.Stdout.String()
		t.Logf("Bootstrap rm all output: %s", output)
		assert.Contains(t, output, "removed auto", "bootstrap rm all should report removing 'auto'")
	})

	t.Run("bootstrap mixed auto and specific peers", func(t *testing.T) {
		t.Parallel()
		// Test that bootstrap commands work when mixing "auto" with specific peers
		node := harness.NewT(t).NewNode().Init("--profile=test")
		node.SetIPFSConfig("AutoConf.Enabled", true)
		node.SetIPFSConfig("Bootstrap", []string{}) // Start with empty bootstrap

		// Add a specific peer first
		specificPeer := "/ip4/127.0.0.1/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ"
		result := node.RunIPFS("bootstrap", "add", specificPeer)
		require.Equal(t, 0, result.ExitCode(), "bootstrap add specific peer should succeed")

		// Add auto placeholder
		result = node.RunIPFS("bootstrap", "add", "auto")
		require.Equal(t, 0, result.ExitCode(), "bootstrap add auto should succeed")

		// Verify bootstrap list shows both
		listResult := node.RunIPFS("bootstrap", "list")
		require.Equal(t, 0, listResult.ExitCode(), "bootstrap list should succeed")

		listOutput := listResult.Stdout.String()
		t.Logf("Bootstrap list with mixed peers: %s", listOutput)
		assert.Contains(t, listOutput, "auto", "bootstrap list should contain 'auto' placeholder")
		assert.Contains(t, listOutput, specificPeer, "bootstrap list should contain specific peer")

		// Try to remove the specific peer - should fail because auto is present
		result = node.RunIPFS("bootstrap", "rm", specificPeer)
		require.NotEqual(t, 0, result.ExitCode(), "bootstrap rm of specific peer should fail when 'auto' is present")

		output := result.Stderr.String()
		assert.Contains(t, output, "cannot remove individual bootstrap peers when using 'auto' placeholder",
			"should provide helpful error message about auto placeholder")

		// Remove all should work and remove both auto and specific peer
		result = node.RunIPFS("bootstrap", "rm", "--all")
		require.Equal(t, 0, result.ExitCode(), "bootstrap rm --all should succeed")

		output = result.Stdout.String()
		t.Logf("Bootstrap rm --all output with mixed peers: %s", output)
		// Should report removing both the specific peer and auto
		assert.Contains(t, output, "removed", "should report removing peers")

		// Verify bootstrap list is now empty
		listResult = node.RunIPFS("bootstrap", "list")
		require.Equal(t, 0, listResult.ExitCode(), "bootstrap list should succeed")

		listOutput = listResult.Stdout.String()
		assert.Empty(t, listOutput, "bootstrap list should be empty after rm --all")
	})
}
