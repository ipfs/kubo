package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigFileOption(t *testing.T) {
	t.Parallel()

	t.Run("daemon uses --config-file option", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		node := h.NewNode().Init()

		// Create a directory outside IPFS_PATH for the config file
		externalConfigDir := filepath.Join(h.Dir, "external-config")
		require.NoError(t, os.MkdirAll(externalConfigDir, 0o755))

		// Copy config to external location
		originalConfigPath := node.ConfigFile()
		externalConfigPath := filepath.Join(externalConfigDir, "config")

		configContent := node.ReadFile(originalConfigPath)
		require.NoError(t, os.WriteFile(externalConfigPath, []byte(configContent), 0o600))

		// Modify the external config to have a distinctive Gateway.RootRedirect
		node.Runner.MustRun(harness.RunRequest{
			Path: node.IPFSBin,
			Args: []string{"config", "--config-file", externalConfigPath, "Gateway.RootRedirect", "/external-config-test"},
		})

		// Verify the original config does not have this value
		originalShow := node.RunIPFS("config", "show")
		assert.NotContains(t, originalShow.Stdout.String(), "/external-config-test")

		// Start daemon with --config-file pointing to external config
		node.StartDaemon("--config-file", externalConfigPath)
		defer node.StopDaemon()

		// Verify daemon is using the external config by checking config show via API
		// The daemon's config show should return the external config's value
		res := node.IPFS("config", "Gateway.RootRedirect")
		assert.Contains(t, res.Stdout.String(), "/external-config-test")
	})

	t.Run("ipfs config show --config-file works with external config", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		node := h.NewNode().Init()

		// Create external config with a distinctive value
		externalConfigDir := filepath.Join(h.Dir, "external-config-show")
		require.NoError(t, os.MkdirAll(externalConfigDir, 0o755))

		// Copy config to external location
		externalConfigPath := filepath.Join(externalConfigDir, "config")
		configContent := node.ReadFile(node.ConfigFile())
		require.NoError(t, os.WriteFile(externalConfigPath, []byte(configContent), 0o600))

		// Modify the external config to have a distinctive value
		node.Runner.MustRun(harness.RunRequest{
			Path: node.IPFSBin,
			Args: []string{"config", "--config-file", externalConfigPath, "Gateway.RootRedirect", "/test-redirect"},
		})

		// Verify the external config was modified
		res := node.Runner.MustRun(harness.RunRequest{
			Path: node.IPFSBin,
			Args: []string{"config", "--config-file", externalConfigPath, "show"},
		})
		assert.Contains(t, res.Stdout.String(), "/test-redirect")

		// Verify the original config was NOT modified
		res = node.RunIPFS("config", "show")
		assert.NotContains(t, res.Stdout.String(), "/test-redirect")
	})

	t.Run("config set uses --config-file", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		node := h.NewNode().Init()

		// Create external config
		externalConfigDir := filepath.Join(h.Dir, "external-config-set")
		require.NoError(t, os.MkdirAll(externalConfigDir, 0o755))

		externalConfigPath := filepath.Join(externalConfigDir, "config")
		configContent := node.ReadFile(node.ConfigFile())
		require.NoError(t, os.WriteFile(externalConfigPath, []byte(configContent), 0o600))

		// Set a distinctive value that we control - set a specific API.HTTPHeaders value
		distinctiveValue := "X-Test-Header"

		// First, set the value in both configs to known initial states
		node.Runner.MustRun(harness.RunRequest{
			Path: node.IPFSBin,
			Args: []string{"config", "--config-file", externalConfigPath, "--json", "API.HTTPHeaders", `{}`},
		})
		node.RunIPFS("config", "--json", "API.HTTPHeaders", `{}`)

		// Verify initial state - neither config has the header
		initialExternal := node.Runner.MustRun(harness.RunRequest{
			Path: node.IPFSBin,
			Args: []string{"config", "--config-file", externalConfigPath, "API.HTTPHeaders"},
		})
		require.NotContains(t, initialExternal.Stdout.String(), distinctiveValue,
			"external config should not have the test header initially")

		initialOriginal := node.RunIPFS("config", "API.HTTPHeaders")
		require.NotContains(t, initialOriginal.Stdout.String(), distinctiveValue,
			"original config should not have the test header initially")

		// Set the distinctive value ONLY in the external config
		node.Runner.MustRun(harness.RunRequest{
			Path: node.IPFSBin,
			Args: []string{"config", "--config-file", externalConfigPath, "--json", "API.HTTPHeaders", `{"` + distinctiveValue + `": ["value"]}`},
		})

		// Verify the external config was modified
		res := node.Runner.MustRun(harness.RunRequest{
			Path: node.IPFSBin,
			Args: []string{"config", "--config-file", externalConfigPath, "API.HTTPHeaders"},
		})
		assert.Contains(t, res.Stdout.String(), distinctiveValue,
			"external config should have the test header after setting")

		// Verify the original config was NOT modified
		res = node.RunIPFS("config", "API.HTTPHeaders")
		assert.NotContains(t, res.Stdout.String(), distinctiveValue,
			"original config should not be modified by --config-file operation")
	})

	t.Run("config profile apply uses --config-file", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		node := h.NewNode().Init()

		// Create external config
		externalConfigDir := filepath.Join(h.Dir, "external-config-profile")
		require.NoError(t, os.MkdirAll(externalConfigDir, 0o755))

		externalConfigPath := filepath.Join(externalConfigDir, "config")
		configContent := node.ReadFile(node.ConfigFile())
		require.NoError(t, os.WriteFile(externalConfigPath, []byte(configContent), 0o600))

		// Set a known initial state: MDNS enabled = true (which local-discovery profile restores)
		// We set it to false initially, then apply local-discovery to set it to true
		node.Runner.MustRun(harness.RunRequest{
			Path: node.IPFSBin,
			Args: []string{"config", "--config-file", externalConfigPath, "--json", "Discovery.MDNS.Enabled", "false"},
		})
		node.RunIPFS("config", "--json", "Discovery.MDNS.Enabled", "false")

		// Verify initial state - both configs have MDNS disabled
		initialExternal := node.Runner.MustRun(harness.RunRequest{
			Path: node.IPFSBin,
			Args: []string{"config", "--config-file", externalConfigPath, "Discovery.MDNS.Enabled"},
		})
		require.Contains(t, initialExternal.Stdout.String(), "false",
			"external config should have MDNS disabled initially")

		initialOriginal := node.RunIPFS("config", "Discovery.MDNS.Enabled")
		require.Contains(t, initialOriginal.Stdout.String(), "false",
			"original config should have MDNS disabled initially")

		// Apply local-discovery profile to external config only
		// This profile sets Discovery.MDNS.Enabled = true
		node.Runner.MustRun(harness.RunRequest{
			Path: node.IPFSBin,
			Args: []string{"config", "--config-file", externalConfigPath, "profile", "apply", "local-discovery"},
		})

		// Verify the external config was modified by the profile
		res := node.Runner.MustRun(harness.RunRequest{
			Path: node.IPFSBin,
			Args: []string{"config", "--config-file", externalConfigPath, "Discovery.MDNS.Enabled"},
		})
		assert.Contains(t, res.Stdout.String(), "true",
			"external config should have MDNS enabled after applying local-discovery profile")

		// Verify the original config was NOT modified - it should still have MDNS disabled
		res = node.RunIPFS("config", "Discovery.MDNS.Enabled")
		assert.Contains(t, res.Stdout.String(), "false",
			"original config should still have MDNS disabled - --config-file should not affect it")
	})

	t.Run("bootstrap commands use --config-file", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		node := h.NewNode().Init()

		// Create external config
		externalConfigDir := filepath.Join(h.Dir, "external-config-bootstrap")
		require.NoError(t, os.MkdirAll(externalConfigDir, 0o755))

		externalConfigPath := filepath.Join(externalConfigDir, "config")
		configContent := node.ReadFile(node.ConfigFile())
		require.NoError(t, os.WriteFile(externalConfigPath, []byte(configContent), 0o600))

		// The test profile sets Bootstrap to empty, so first we need to add some peers
		// to have something to verify. We'll add a known test peer to both configs.
		testPeer := "/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN"

		// Add test peer to external config
		node.Runner.MustRun(harness.RunRequest{
			Path: node.IPFSBin,
			Args: []string{"bootstrap", "--config-file", externalConfigPath, "add", testPeer},
		})

		// Add test peer to original config
		node.RunIPFS("bootstrap", "add", testPeer)

		// Verify both configs have the test peer
		externalList := node.Runner.MustRun(harness.RunRequest{
			Path: node.IPFSBin,
			Args: []string{"bootstrap", "--config-file", externalConfigPath, "list"},
		})
		require.Contains(t, externalList.Stdout.String(), testPeer,
			"external config should have the test bootstrap peer")

		originalList := node.RunIPFS("bootstrap", "list")
		require.Contains(t, originalList.Stdout.String(), testPeer,
			"original config should have the test bootstrap peer")

		// Remove all bootstrap peers from external config only
		node.Runner.MustRun(harness.RunRequest{
			Path: node.IPFSBin,
			Args: []string{"bootstrap", "--config-file", externalConfigPath, "rm", "all"},
		})

		// Verify the external config now has no bootstrap peers
		res := node.Runner.MustRun(harness.RunRequest{
			Path: node.IPFSBin,
			Args: []string{"bootstrap", "--config-file", externalConfigPath, "list"},
		})
		assert.Empty(t, res.Stdout.String(),
			"external config should have no bootstrap peers after 'rm all'")

		// Verify the original config was NOT modified - it should still have the peer
		res = node.RunIPFS("bootstrap", "list")
		assert.Contains(t, res.Stdout.String(), testPeer,
			"original config should still have bootstrap peer - --config-file should not affect it")
	})

	t.Run("init with --config-file writes to custom location", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)

		// Create directories for repo and config
		repoDir := filepath.Join(h.Dir, "repo")
		configDir := filepath.Join(h.Dir, "config-dir")
		require.NoError(t, os.MkdirAll(configDir, 0o755))

		externalConfigPath := filepath.Join(configDir, "my-config")

		// Initialize with --config-file
		h.Runner.MustRun(harness.RunRequest{
			Path: h.IPFSBin,
			Args: []string{"init", "--repo-dir", repoDir, "--config-file", externalConfigPath},
		})

		// Verify config was written to the external location
		_, err := os.Stat(externalConfigPath)
		require.NoError(t, err, "config should exist at external path")

		// Verify config is NOT in repo dir
		_, err = os.Stat(filepath.Join(repoDir, "config"))
		require.True(t, os.IsNotExist(err), "config should NOT exist in repo dir")

		// Verify datastore IS in repo dir
		_, err = os.Stat(filepath.Join(repoDir, "datastore"))
		require.NoError(t, err, "datastore should exist in repo dir")

		// Verify keystore IS in repo dir
		_, err = os.Stat(filepath.Join(repoDir, "keystore"))
		require.NoError(t, err, "keystore should exist in repo dir")
	})

	t.Run("separation of config and repo paths", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)

		// Create separate directories
		repoDir := filepath.Join(h.Dir, "repo-separate")
		configDir := filepath.Join(h.Dir, "config-separate")
		require.NoError(t, os.MkdirAll(configDir, 0o755))

		externalConfigPath := filepath.Join(configDir, "config")

		// Initialize with both --repo-dir and --config-file
		h.Runner.MustRun(harness.RunRequest{
			Path: h.IPFSBin,
			Args: []string{"init", "--repo-dir", repoDir, "--config-file", externalConfigPath},
		})

		// Verify file locations
		// Config should ONLY be at external path
		_, err := os.Stat(externalConfigPath)
		require.NoError(t, err, "config should exist at external path")

		_, err = os.Stat(filepath.Join(repoDir, "config"))
		require.True(t, os.IsNotExist(err), "config should NOT exist in repo dir")

		// Datastore spec should be in repo dir
		_, err = os.Stat(filepath.Join(repoDir, "datastore_spec"))
		require.NoError(t, err, "datastore_spec should exist in repo dir")

		// Version file should be in repo dir
		_, err = os.Stat(filepath.Join(repoDir, "version"))
		require.NoError(t, err, "version should exist in repo dir")

		// Keystore should be in repo dir
		_, err = os.Stat(filepath.Join(repoDir, "keystore"))
		require.NoError(t, err, "keystore should exist in repo dir")
	})

	// Kubernetes scenario: config from ConfigMap, init creates repo infrastructure
	// This tests the case where a config file is pre-populated (e.g., from a ConfigMap)
	// and the repo needs to be initialized using that config.
	t.Run("init with pre-existing config creates repo infrastructure", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)

		// Create directories - simulating a Kubernetes pod with:
		// - ConfigMap mounted config file
		// - Empty persistent volume for repo
		configDir := filepath.Join(h.Dir, "configmap")
		repoDir := filepath.Join(h.Dir, "repo-k8s")
		require.NoError(t, os.MkdirAll(configDir, 0o755))

		externalConfigPath := filepath.Join(configDir, "config")

		// First, create a valid config file elsewhere (simulating a ConfigMap)
		tempInitDir := filepath.Join(h.Dir, "temp-init")
		h.Runner.MustRun(harness.RunRequest{
			Path: h.IPFSBin,
			Args: []string{"init", "--repo-dir", tempInitDir},
		})

		// Get the PeerID from the temp config for later verification
		tempIDRes := h.Runner.MustRun(harness.RunRequest{
			Path: h.IPFSBin,
			Args: []string{"--repo-dir", tempInitDir, "config", "Identity.PeerID"},
		})
		expectedPeerID := tempIDRes.Stdout.Trimmed()
		require.NotEmpty(t, expectedPeerID)

		// Copy config to "ConfigMap" location (simulating how Kubernetes mounts ConfigMaps)
		configContent, err := os.ReadFile(filepath.Join(tempInitDir, "config"))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(externalConfigPath, configContent, 0o600))

		// Remove temp init dir - we only needed it to generate a valid config
		require.NoError(t, os.RemoveAll(tempInitDir))

		// Verify starting state: config exists, repo dir does not exist
		_, err = os.Stat(externalConfigPath)
		require.NoError(t, err, "external config should exist (simulating ConfigMap)")

		_, err = os.Stat(repoDir)
		require.True(t, os.IsNotExist(err), "repo dir should not exist yet")

		// Initialize repo with pre-existing config
		// This simulates: ipfs init --repo-dir /data/ipfs --config-file /etc/ipfs/config
		h.Runner.MustRun(harness.RunRequest{
			Path: h.IPFSBin,
			Args: []string{"init", "--repo-dir", repoDir, "--config-file", externalConfigPath},
		})

		// Verify repo infrastructure was created in repo dir
		_, err = os.Stat(filepath.Join(repoDir, "datastore"))
		require.NoError(t, err, "datastore should exist in repo dir")

		_, err = os.Stat(filepath.Join(repoDir, "keystore"))
		require.NoError(t, err, "keystore should exist in repo dir")

		_, err = os.Stat(filepath.Join(repoDir, "version"))
		require.NoError(t, err, "version should exist in repo dir")

		_, err = os.Stat(filepath.Join(repoDir, "datastore_spec"))
		require.NoError(t, err, "datastore_spec should exist in repo dir")

		// Verify config is NOT in repo dir (should only be at external location)
		_, err = os.Stat(filepath.Join(repoDir, "config"))
		require.True(t, os.IsNotExist(err), "config should NOT exist in repo dir - only at external path")

		// Verify the pre-existing config was NOT overwritten (check PeerID matches)
		actualIDRes := h.Runner.MustRun(harness.RunRequest{
			Path: h.IPFSBin,
			Args: []string{"--repo-dir", repoDir, "--config-file", externalConfigPath, "config", "Identity.PeerID"},
		})
		assert.Equal(t, expectedPeerID, actualIDRes.Stdout.Trimmed(),
			"pre-existing config should not be overwritten - PeerID should match")
	})

	// Test --init-config flag (daemon's template copy behavior)
	t.Run("daemon --init --init-config copies template to repo", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)

		// Create a config template with a distinctive value
		// Apply test profile to use random ports and avoid conflicts
		templateDir := filepath.Join(h.Dir, "template")
		h.Runner.MustRun(harness.RunRequest{
			Path: h.IPFSBin,
			Args: []string{"init", "--repo-dir", templateDir, "--profile=test"},
		})

		templatePath := filepath.Join(templateDir, "config")

		// Set distinctive value in template
		h.Runner.MustRun(harness.RunRequest{
			Path: h.IPFSBin,
			Args: []string{"--repo-dir", templateDir, "config", "Gateway.RootRedirect", "/template-value"},
		})

		// Create a new node that will use --init-config
		node := h.NewNode()

		// Start daemon with --init --init-config (copies template to node's repo)
		// Use --init-profile=randomports to ensure unique ports
		node.StartDaemon("--init", "--init-config", templatePath, "--init-profile=randomports")
		defer node.StopDaemon()

		// Verify config was COPIED to node's repo dir
		_, err := os.Stat(filepath.Join(node.Dir, "config"))
		require.NoError(t, err, "config should be copied to repo dir when using --init-config")

		// Verify the value from template is present
		res := node.IPFS("config", "Gateway.RootRedirect")
		assert.Contains(t, res.Stdout.String(), "/template-value",
			"daemon should use values from the --init-config template")
	})

	t.Run("--init-config preserves Identity from template", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)

		// Create a config template and get its PeerID
		templateDir := filepath.Join(h.Dir, "template-identity")
		h.Runner.MustRun(harness.RunRequest{
			Path: h.IPFSBin,
			Args: []string{"init", "--repo-dir", templateDir},
		})

		templatePath := filepath.Join(templateDir, "config")

		// Get the PeerID from the template
		templateIDRes := h.Runner.MustRun(harness.RunRequest{
			Path: h.IPFSBin,
			Args: []string{"--repo-dir", templateDir, "config", "Identity.PeerID"},
		})
		expectedPeerID := templateIDRes.Stdout.Trimmed()
		require.NotEmpty(t, expectedPeerID)

		// Create a new node that will use --init-config
		node := h.NewNode()

		// Start daemon with --init --init-config
		node.StartDaemon("--init", "--init-config", templatePath)
		defer node.StopDaemon()

		// Verify the PeerID matches the template (not a newly generated one)
		actualPeerID := node.PeerID().String()
		assert.Equal(t, expectedPeerID, actualPeerID,
			"--init-config should preserve Identity from template, not generate new keypair")
	})

	// This test demonstrates the key behavioral difference between the two flags
	t.Run("--init-config copies once vs --config-file references directly", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)

		// Part A: Test --init-config (one-time copy behavior)
		// Note: --init-config is a daemon flag, not an init flag
		t.Run("--init-config is a one-time copy", func(t *testing.T) {
			// Create template with initial value (use test profile for random ports)
			templateDir := filepath.Join(h.Dir, "template-copy")
			h.Runner.MustRun(harness.RunRequest{
				Path: h.IPFSBin,
				Args: []string{"init", "--repo-dir", templateDir, "--profile=test"},
			})
			templatePath := filepath.Join(templateDir, "config")

			h.Runner.MustRun(harness.RunRequest{
				Path: h.IPFSBin,
				Args: []string{"--repo-dir", templateDir, "config", "Gateway.RootRedirect", "/value-A"},
			})

			// Create new node and start daemon with --init --init-config
			node := h.NewNode()
			node.StartDaemon("--init", "--init-config", templatePath, "--init-profile=randomports")

			// Verify the value from template is copied
			res := node.IPFS("config", "Gateway.RootRedirect")
			require.Contains(t, res.Stdout.String(), "/value-A")

			node.StopDaemon()

			// Now modify the template to /value-B
			h.Runner.MustRun(harness.RunRequest{
				Path: h.IPFSBin,
				Args: []string{"--repo-dir", templateDir, "config", "Gateway.RootRedirect", "/value-B"},
			})

			// Restart daemon - it should still see /value-A (from the copy)
			node.StartDaemon()
			defer node.StopDaemon()

			res = node.IPFS("config", "Gateway.RootRedirect")
			assert.Contains(t, res.Stdout.String(), "/value-A",
				"--init-config copies once; changes to template after init should have no effect")
			assert.NotContains(t, res.Stdout.String(), "/value-B",
				"repo should not see template changes after init")
		})

		// Part B: Test --config-file (persistent reference behavior)
		t.Run("--config-file references directly", func(t *testing.T) {
			// Create external config with initial value
			configDir := filepath.Join(h.Dir, "external-ref")
			repoDir := filepath.Join(h.Dir, "repo-config-file")
			require.NoError(t, os.MkdirAll(configDir, 0o755))
			externalConfigPath := filepath.Join(configDir, "config")

			// Initialize to create the config at external path
			h.Runner.MustRun(harness.RunRequest{
				Path: h.IPFSBin,
				Args: []string{"init", "--repo-dir", repoDir, "--config-file", externalConfigPath},
			})

			// Set initial value
			h.Runner.MustRun(harness.RunRequest{
				Path: h.IPFSBin,
				Args: []string{"--repo-dir", repoDir, "--config-file", externalConfigPath, "config", "Gateway.RootRedirect", "/value-A"},
			})

			// Verify initial value
			res := h.Runner.MustRun(harness.RunRequest{
				Path: h.IPFSBin,
				Args: []string{"--repo-dir", repoDir, "--config-file", externalConfigPath, "config", "Gateway.RootRedirect"},
			})
			require.Contains(t, res.Stdout.String(), "/value-A")

			// Update external config to /value-B
			h.Runner.MustRun(harness.RunRequest{
				Path: h.IPFSBin,
				Args: []string{"--repo-dir", repoDir, "--config-file", externalConfigPath, "config", "Gateway.RootRedirect", "/value-B"},
			})

			// Verify we now see /value-B (config is referenced directly, not copied)
			res = h.Runner.MustRun(harness.RunRequest{
				Path: h.IPFSBin,
				Args: []string{"--repo-dir", repoDir, "--config-file", externalConfigPath, "config", "Gateway.RootRedirect"},
			})
			assert.Contains(t, res.Stdout.String(), "/value-B",
				"--config-file should reference config directly; changes should be visible immediately")
		})
	})

	t.Run("commands work with --repo-dir and --config-file together", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)

		// Create separate directories for repo and config
		repoDir := filepath.Join(h.Dir, "repo-combined")
		configDir := filepath.Join(h.Dir, "config-combined")
		require.NoError(t, os.MkdirAll(configDir, 0o755))
		externalConfigPath := filepath.Join(configDir, "config")

		// Initialize with both flags
		h.Runner.MustRun(harness.RunRequest{
			Path: h.IPFSBin,
			Args: []string{"init", "--repo-dir", repoDir, "--config-file", externalConfigPath},
		})

		// Verify repo infrastructure is in repoDir
		_, err := os.Stat(filepath.Join(repoDir, "datastore"))
		require.NoError(t, err, "datastore should be in repo dir")
		_, err = os.Stat(filepath.Join(repoDir, "keystore"))
		require.NoError(t, err, "keystore should be in repo dir")

		// Verify config is NOT in repo dir
		_, err = os.Stat(filepath.Join(repoDir, "config"))
		require.True(t, os.IsNotExist(err), "config should NOT exist in repo dir")

		// Verify config IS at external path
		_, err = os.Stat(externalConfigPath)
		require.NoError(t, err, "config should exist at external path")

		// Set a distinctive value
		h.Runner.MustRun(harness.RunRequest{
			Path: h.IPFSBin,
			Args: []string{"--repo-dir", repoDir, "--config-file", externalConfigPath, "config", "Gateway.RootRedirect", "/combined-test"},
		})

		// Verify config read works with both flags
		res := h.Runner.MustRun(harness.RunRequest{
			Path: h.IPFSBin,
			Args: []string{"--repo-dir", repoDir, "--config-file", externalConfigPath, "config", "Gateway.RootRedirect"},
		})
		assert.Contains(t, res.Stdout.String(), "/combined-test",
			"config should be read from --config-file path")

		// Get PeerID to verify identity was created
		idRes := h.Runner.MustRun(harness.RunRequest{
			Path: h.IPFSBin,
			Args: []string{"--repo-dir", repoDir, "--config-file", externalConfigPath, "config", "Identity.PeerID"},
		})
		peerID := idRes.Stdout.Trimmed()
		assert.NotEmpty(t, peerID, "PeerID should be set in config")

		// Verify ipfs id works offline with both flags
		idRunRes := h.Runner.MustRun(harness.RunRequest{
			Path: h.IPFSBin,
			Args: []string{"--repo-dir", repoDir, "--config-file", externalConfigPath, "id", "--offline"},
		})
		assert.Contains(t, idRunRes.Stdout.String(), peerID,
			"ipfs id --offline should work with --repo-dir and --config-file")

		// Verify bootstrap command works with both flags
		h.Runner.MustRun(harness.RunRequest{
			Path: h.IPFSBin,
			Args: []string{"--repo-dir", repoDir, "--config-file", externalConfigPath, "bootstrap", "list"},
		})
	})
}
