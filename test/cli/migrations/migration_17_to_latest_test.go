package migrations

// NOTE: These migration tests require the local Kubo binary (built with 'make build') to be in PATH.
//
// To run these tests successfully:
//   export PATH="$(pwd)/cmd/ipfs:$PATH"
//   go test ./test/cli/migrations/

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ipfs "github.com/ipfs/kubo"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/require"
)

// TestMigration17ToLatest tests migration from repo version 17 to the latest version.
//
// Since we don't have a v17 repo fixture, we start with v16 and migrate it to v17 first,
// then test the 17-to-18 migration specifically.
//
// This test focuses on the Provider/Reprovider to Provide consolidation that happens in 17-to-18.
func TestMigration17ToLatest(t *testing.T) {
	t.Parallel()

	// Tests for Provider/Reprovider to Provide migration (17-to-18)
	t.Run("daemon migrate: Provider/Reprovider to Provide consolidation", testProviderReproviderMigration)
	t.Run("daemon migrate: flat strategy conversion", testFlatStrategyConversion)
	t.Run("repo migrate: Provider/Reprovider to Provide consolidation", testRepoProviderReproviderMigration)
}

// =============================================================================
// MIGRATION 17-to-18 SPECIFIC TESTS: Provider/Reprovider to Provide consolidation
// =============================================================================

func testProviderReproviderMigration(t *testing.T) {
	// TEST: 17-to-18 migration with explicit Provider/Reprovider configuration
	node := setupV17RepoWithProviderConfig(t)

	configPath := filepath.Join(node.Dir, "config")
	versionPath := filepath.Join(node.Dir, "version")

	// Run migration using daemon --migrate command
	stdoutOutput, migrationSuccess := runDaemonMigrationFromV17(t, node)

	// Debug: Print the actual output
	t.Logf("Daemon output:\n%s", stdoutOutput)

	// Verify migration was successful
	require.True(t, migrationSuccess, "Migration should have been successful")
	require.Contains(t, stdoutOutput, "applying 17-to-18 repo migration", "Migration 17-to-18 should have been triggered")
	require.Contains(t, stdoutOutput, "Migration 17-to-18 succeeded", "Migration 17-to-18 should have completed successfully")

	// Verify version was updated to latest
	versionData, err := os.ReadFile(versionPath)
	require.NoError(t, err)
	expectedVersion := fmt.Sprint(ipfs.RepoVersion)
	require.Equal(t, expectedVersion, strings.TrimSpace(string(versionData)), "Version should be updated to %s (latest)", expectedVersion)

	// =============================================================================
	// MIGRATION 17-to-18 ASSERTIONS: Provider/Reprovider to Provide consolidation
	// =============================================================================
	helper := NewMigrationTestHelper(t, configPath)

	// Verify Provider/Reprovider migration to Provide
	helper.RequireProviderMigration().
		RequireFieldEquals("Provide.Enabled", true).           // Migrated from Provider.Enabled
		RequireFieldEquals("Provide.WorkerCount", float64(8)). // Migrated from Provider.WorkerCount
		RequireFieldEquals("Provide.Strategy", "roots").       // Migrated from Reprovider.Strategy
		RequireFieldEquals("Provide.Interval", "24h")          // Migrated from Reprovider.Interval

	// Verify old sections are removed
	helper.RequireFieldAbsent("Provider").
		RequireFieldAbsent("Reprovider")
}

func testFlatStrategyConversion(t *testing.T) {
	// TEST: 17-to-18 migration with "flat" strategy that should convert to "all"
	node := setupV17RepoWithFlatStrategy(t)

	configPath := filepath.Join(node.Dir, "config")

	// Run migration using daemon --migrate command
	stdoutOutput, migrationSuccess := runDaemonMigrationFromV17(t, node)

	// Verify migration was successful
	require.True(t, migrationSuccess, "Migration should have been successful")
	require.Contains(t, stdoutOutput, "applying 17-to-18 repo migration", "Migration 17-to-18 should have been triggered")
	require.Contains(t, stdoutOutput, "Migration 17-to-18 succeeded", "Migration 17-to-18 should have completed successfully")

	// =============================================================================
	// MIGRATION 17-to-18 ASSERTIONS: "flat" to "all" strategy conversion
	// =============================================================================
	helper := NewMigrationTestHelper(t, configPath)

	// Verify "flat" was converted to "all"
	helper.RequireProviderMigration().
		RequireFieldEquals("Provide.Strategy", "all"). // "flat" converted to "all"
		RequireFieldEquals("Provide.Interval", "12h")
}

func testRepoProviderReproviderMigration(t *testing.T) {
	// TEST: 17-to-18 migration using 'ipfs repo migrate' command
	node := setupV17RepoWithProviderConfig(t)

	configPath := filepath.Join(node.Dir, "config")

	// Run migration using 'ipfs repo migrate' command
	result := node.RunIPFS("repo", "migrate")
	require.Empty(t, result.Stderr.String(), "Migration should succeed without errors")

	// Verify same results as daemon migrate
	helper := NewMigrationTestHelper(t, configPath)
	helper.RequireProviderMigration().
		RequireFieldEquals("Provide.Enabled", true).
		RequireFieldEquals("Provide.WorkerCount", float64(8)).
		RequireFieldEquals("Provide.Strategy", "roots").
		RequireFieldEquals("Provide.Interval", "24h")
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// setupV17RepoWithProviderConfig creates a v17 repo with Provider/Reprovider configuration
func setupV17RepoWithProviderConfig(t *testing.T) *harness.Node {
	// Start with v16 repo and migrate to v17 first
	node := setupStaticV16Repo(t)

	// First migrate to v17
	result := node.RunIPFS("repo", "migrate", "--to=17")
	require.Empty(t, result.Stderr.String(), "Migration to v17 should succeed")

	// Add Provider and Reprovider configuration
	configPath := filepath.Join(node.Dir, "config")
	var config map[string]interface{}
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(configData, &config))

	config["Provider"] = map[string]interface{}{
		"Enabled":     true,
		"WorkerCount": 8,
	}
	config["Reprovider"] = map[string]interface{}{
		"Strategy": "roots",
		"Interval": "24h",
	}

	modifiedConfigData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, modifiedConfigData, 0644))

	return node
}

// setupV17RepoWithFlatStrategy creates a v17 repo with "flat" strategy for testing conversion
func setupV17RepoWithFlatStrategy(t *testing.T) *harness.Node {
	// Start with v16 repo and migrate to v17 first
	node := setupStaticV16Repo(t)

	// First migrate to v17
	result := node.RunIPFS("repo", "migrate", "--to=17")
	require.Empty(t, result.Stderr.String(), "Migration to v17 should succeed")

	// Add Provider and Reprovider configuration with "flat" strategy
	configPath := filepath.Join(node.Dir, "config")
	var config map[string]interface{}
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(configData, &config))

	config["Provider"] = map[string]interface{}{
		"Enabled": false,
	}
	config["Reprovider"] = map[string]interface{}{
		"Strategy": "flat", // This should be converted to "all"
		"Interval": "12h",
	}

	modifiedConfigData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, modifiedConfigData, 0644))

	return node
}

// runDaemonMigrationFromV17 monitors daemon startup for 17-to-18 migration only
func runDaemonMigrationFromV17(t *testing.T, node *harness.Node) (string, bool) {
	// Monitor only the 17-to-18 migration
	expectedMigrations := []struct {
		pattern string
		success string
	}{
		{
			pattern: "applying 17-to-18 repo migration",
			success: "Migration 17-to-18 succeeded",
		},
	}

	return runDaemonWithMultipleMigrationMonitoring(t, node, expectedMigrations)
}

// RequireProviderMigration verifies that Provider/Reprovider have been migrated to Provide section
func (h *MigrationTestHelper) RequireProviderMigration() *MigrationTestHelper {
	return h.RequireFieldExists("Provide").
		RequireFieldAbsent("Provider").
		RequireFieldAbsent("Reprovider")
}
