package cli

import (
	"strings"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/sjson"
)

func TestConfigSecrets(t *testing.T) {
	t.Parallel()

	t.Run("Identity.PrivKey protection", func(t *testing.T) {
		t.Parallel()

		t.Run("Identity.PrivKey is concealed in config show", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init()

			// Read the actual config file to get the real PrivKey
			configFile := node.ReadFile(node.ConfigFile())
			assert.Contains(t, configFile, "PrivKey")

			// config show should NOT contain the PrivKey
			configShow := node.RunIPFS("config", "show").Stdout.String()
			assert.NotContains(t, configShow, "PrivKey")
		})

		t.Run("Identity.PrivKey cannot be read via ipfs config", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init()

			// Attempting to read Identity.PrivKey should fail
			res := node.RunIPFS("config", "Identity.PrivKey")
			assert.Equal(t, 1, res.ExitCode())
			assert.Contains(t, res.Stderr.String(), "cannot show or change private key")
		})

		t.Run("Identity.PrivKey cannot be read via ipfs config Identity", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init()

			// Attempting to read Identity section should fail (it contains PrivKey)
			res := node.RunIPFS("config", "Identity")
			assert.Equal(t, 1, res.ExitCode())
			assert.Contains(t, res.Stderr.String(), "cannot show or change private key")
		})

		t.Run("Identity.PrivKey cannot be set via config replace", func(t *testing.T) {
			t.Parallel()
			// Key rotation must be done in offline mode via the dedicated `ipfs key rotate` command.
			// This test ensures PrivKey cannot be changed via config replace.
			node := harness.NewT(t).NewNode().Init()

			configShow := node.RunIPFS("config", "show").Stdout.String()

			// Try to inject a PrivKey via config replace
			configJSON := MustVal(sjson.Set(configShow, "Identity.PrivKey", "CAASqAkwggSkAgEAAo"))
			node.WriteBytes("new-config", []byte(configJSON))
			res := node.RunIPFS("config", "replace", "new-config")
			assert.Equal(t, 1, res.ExitCode())
			assert.Contains(t, res.Stderr.String(), "setting private key")
		})

		t.Run("Identity.PrivKey is preserved when re-injecting config", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init()

			// Read the original config file
			originalConfig := node.ReadFile(node.ConfigFile())
			assert.Contains(t, originalConfig, "PrivKey")

			// Extract the PrivKey value for comparison
			var origPrivKey string
			assert.Contains(t, originalConfig, "PrivKey")
			// Simple extraction - find the PrivKey line
			for _, line := range strings.Split(originalConfig, "\n") {
				if strings.Contains(line, "\"PrivKey\":") {
					origPrivKey = line
					break
				}
			}
			assert.NotEmpty(t, origPrivKey)

			// Get config show output (which should NOT contain PrivKey)
			configShow := node.RunIPFS("config", "show").Stdout.String()
			assert.NotContains(t, configShow, "PrivKey")

			// Re-inject the config via config replace
			node.WriteBytes("config-show", []byte(configShow))
			node.IPFS("config", "replace", "config-show")

			// The PrivKey should still be in the config file
			newConfig := node.ReadFile(node.ConfigFile())
			assert.Contains(t, newConfig, "PrivKey")

			// Verify the PrivKey line is the same
			var newPrivKey string
			for _, line := range strings.Split(newConfig, "\n") {
				if strings.Contains(line, "\"PrivKey\":") {
					newPrivKey = line
					break
				}
			}
			assert.Equal(t, origPrivKey, newPrivKey, "PrivKey should be preserved")
		})
	})

	t.Run("TLS security validation", func(t *testing.T) {
		t.Parallel()

		t.Run("AutoConf.TLSInsecureSkipVerify defaults to false", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init()

			// Check the default value in a fresh init
			res := node.RunIPFS("config", "AutoConf.TLSInsecureSkipVerify")
			// Field may not exist (exit code 1) or be false/empty (exit code 0)
			// Both are acceptable as they mean "not true"
			output := res.Stdout.String()
			assert.NotContains(t, output, "true", "default should not be true")
		})

		t.Run("AutoConf.TLSInsecureSkipVerify can be set to true", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init()

			// Set to true
			node.IPFS("config", "AutoConf.TLSInsecureSkipVerify", "true", "--json")

			// Verify it was set
			res := node.RunIPFS("config", "AutoConf.TLSInsecureSkipVerify")
			assert.Equal(t, 0, res.ExitCode())
			assert.Contains(t, res.Stdout.String(), "true")
		})

		t.Run("HTTPRetrieval.TLSInsecureSkipVerify defaults to false", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init()

			// Check the default value in a fresh init
			res := node.RunIPFS("config", "HTTPRetrieval.TLSInsecureSkipVerify")
			// Field may not exist (exit code 1) or be false/empty (exit code 0)
			// Both are acceptable as they mean "not true"
			output := res.Stdout.String()
			assert.NotContains(t, output, "true", "default should not be true")
		})

		t.Run("HTTPRetrieval.TLSInsecureSkipVerify can be set to true", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init()

			// Set to true
			node.IPFS("config", "HTTPRetrieval.TLSInsecureSkipVerify", "true", "--json")

			// Verify it was set
			res := node.RunIPFS("config", "HTTPRetrieval.TLSInsecureSkipVerify")
			assert.Equal(t, 0, res.ExitCode())
			assert.Contains(t, res.Stdout.String(), "true")
		})
	})
}
