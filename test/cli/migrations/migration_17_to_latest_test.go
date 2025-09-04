package migrations

// NOTE: These migration tests require the local Kubo binary (built with 'make build') to be in PATH.
//
// To run these tests successfully:
//   export PATH="$(pwd)/cmd/ipfs:$PATH"
//   go test ./test/cli/migrations/

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	t.Run("daemon migrate: empty Provider/Reprovider sections", testEmptyProviderReproviderMigration)
	t.Run("daemon migrate: partial configuration (Provider only)", testProviderOnlyMigration)
	t.Run("daemon migrate: partial configuration (Reprovider only)", testReproviderOnlyMigration)
	t.Run("repo migrate: invalid strategy values preserved", testInvalidStrategyMigration)
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
		RequireFieldEquals("Provide.Enabled", true).              // Migrated from Provider.Enabled
		RequireFieldEquals("Provide.DHT.MaxWorkers", float64(8)). // Migrated from Provider.WorkerCount
		RequireFieldEquals("Provide.Strategy", "roots").          // Migrated from Reprovider.Strategy
		RequireFieldEquals("Provide.DHT.Interval", "24h")         // Migrated from Reprovider.Interval

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
		RequireFieldEquals("Provide.DHT.Interval", "12h")
}

func testEmptyProviderReproviderMigration(t *testing.T) {
	// TEST: 17-to-18 migration with empty Provider and Reprovider sections
	node := setupV17RepoWithEmptySections(t)

	configPath := filepath.Join(node.Dir, "config")

	// Run migration
	stdoutOutput, migrationSuccess := runDaemonMigrationFromV17(t, node)

	// Verify migration was successful
	require.True(t, migrationSuccess, "Migration should have been successful")
	require.Contains(t, stdoutOutput, "Migration 17-to-18 succeeded")

	// Verify empty sections are removed and no Provide section is created
	helper := NewMigrationTestHelper(t, configPath)
	helper.RequireFieldAbsent("Provider").
		RequireFieldAbsent("Reprovider").
		RequireFieldAbsent("Provide") // No Provide section should be created for empty configs
}

func testProviderOnlyMigration(t *testing.T) {
	// TEST: 17-to-18 migration with only Provider configuration
	node := setupV17RepoWithProviderOnly(t)

	configPath := filepath.Join(node.Dir, "config")

	// Run migration
	stdoutOutput, migrationSuccess := runDaemonMigrationFromV17(t, node)

	// Verify migration was successful
	require.True(t, migrationSuccess, "Migration should have been successful")
	require.Contains(t, stdoutOutput, "Migration 17-to-18 succeeded")

	// Verify only Provider fields are migrated
	helper := NewMigrationTestHelper(t, configPath)
	helper.RequireProviderMigration().
		RequireFieldEquals("Provide.Enabled", false).
		RequireFieldEquals("Provide.DHT.MaxWorkers", float64(32)).
		RequireFieldAbsent("Provide.Strategy").    // No Reprovider.Strategy to migrate
		RequireFieldAbsent("Provide.DHT.Interval") // No Reprovider.Interval to migrate
}

func testReproviderOnlyMigration(t *testing.T) {
	// TEST: 17-to-18 migration with only Reprovider configuration
	node := setupV17RepoWithReproviderOnly(t)

	configPath := filepath.Join(node.Dir, "config")

	// Run migration
	stdoutOutput, migrationSuccess := runDaemonMigrationFromV17(t, node)

	// Verify migration was successful
	require.True(t, migrationSuccess, "Migration should have been successful")
	require.Contains(t, stdoutOutput, "Migration 17-to-18 succeeded")

	// Verify only Reprovider fields are migrated
	helper := NewMigrationTestHelper(t, configPath)
	helper.RequireProviderMigration().
		RequireFieldEquals("Provide.Strategy", "pinned").
		RequireFieldEquals("Provide.DHT.Interval", "48h").
		RequireFieldAbsent("Provide.Enabled").       // No Provider.Enabled to migrate
		RequireFieldAbsent("Provide.DHT.MaxWorkers") // No Provider.WorkerCount to migrate
}

func testInvalidStrategyMigration(t *testing.T) {
	// TEST: 17-to-18 migration with invalid strategy values (should be preserved as-is)
	// The migration itself should succeed, but daemon start will fail due to invalid strategy
	node := setupV17RepoWithInvalidStrategy(t)

	configPath := filepath.Join(node.Dir, "config")

	// Run the migration using 'ipfs repo migrate' (not daemon --migrate)
	// because daemon would fail to start with invalid strategy after migration
	result := node.RunIPFS("repo", "migrate")
	require.Empty(t, result.Stderr.String(), "Migration should succeed without errors")

	// Verify invalid strategy is preserved as-is (not validated during migration)
	helper := NewMigrationTestHelper(t, configPath)
	helper.RequireProviderMigration().
		RequireFieldEquals("Provide.Strategy", "invalid-strategy") // Should be preserved

	// Now verify that daemon fails to start with invalid strategy
	// Note: We cannot use --offline as it skips provider validation
	// Use a context with timeout to avoid hanging
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, node.IPFSBin, "daemon")
	cmd.Dir = node.Dir
	for k, v := range node.Runner.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	output, err := cmd.CombinedOutput()

	// The daemon should fail (either with error or timeout if it's hanging)
	require.Error(t, err, "Daemon should fail to start with invalid strategy")

	// Check if we got the expected error message
	outputStr := string(output)
	t.Logf("Daemon output with invalid strategy: %s", outputStr)

	// The error should mention unknown strategy
	require.Contains(t, outputStr, "unknown strategy", "Should report unknown strategy error")
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
		RequireFieldEquals("Provide.DHT.MaxWorkers", float64(8)).
		RequireFieldEquals("Provide.Strategy", "roots").
		RequireFieldEquals("Provide.DHT.Interval", "24h")
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// setupV17RepoWithProviderConfig creates a v17 repo with Provider/Reprovider configuration
func setupV17RepoWithProviderConfig(t *testing.T) *harness.Node {
	return setupV17RepoWithConfig(t,
		map[string]interface{}{
			"Enabled":     true,
			"WorkerCount": 8,
		},
		map[string]interface{}{
			"Strategy": "roots",
			"Interval": "24h",
		})
}

// setupV17RepoWithFlatStrategy creates a v17 repo with "flat" strategy for testing conversion
func setupV17RepoWithFlatStrategy(t *testing.T) *harness.Node {
	return setupV17RepoWithConfig(t,
		map[string]interface{}{
			"Enabled": false,
		},
		map[string]interface{}{
			"Strategy": "flat", // This should be converted to "all"
			"Interval": "12h",
		})
}

// setupV17RepoWithConfig is a helper that creates a v17 repo with specified Provider/Reprovider config
func setupV17RepoWithConfig(t *testing.T, providerConfig, reproviderConfig map[string]interface{}) *harness.Node {
	node := setupStaticV16Repo(t)

	// First migrate to v17
	result := node.RunIPFS("repo", "migrate", "--to=17")
	require.Empty(t, result.Stderr.String(), "Migration to v17 should succeed")

	// Update config with specified Provider and Reprovider settings
	configPath := filepath.Join(node.Dir, "config")
	var config map[string]interface{}
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(configData, &config))

	if providerConfig != nil {
		config["Provider"] = providerConfig
	} else {
		config["Provider"] = map[string]interface{}{}
	}

	if reproviderConfig != nil {
		config["Reprovider"] = reproviderConfig
	} else {
		config["Reprovider"] = map[string]interface{}{}
	}

	modifiedConfigData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, modifiedConfigData, 0644))

	return node
}

// setupV17RepoWithEmptySections creates a v17 repo with empty Provider/Reprovider sections
func setupV17RepoWithEmptySections(t *testing.T) *harness.Node {
	return setupV17RepoWithConfig(t,
		map[string]interface{}{},
		map[string]interface{}{})
}

// setupV17RepoWithProviderOnly creates a v17 repo with only Provider configuration
func setupV17RepoWithProviderOnly(t *testing.T) *harness.Node {
	return setupV17RepoWithConfig(t,
		map[string]interface{}{
			"Enabled":     false,
			"WorkerCount": 32,
		},
		map[string]interface{}{})
}

// setupV17RepoWithReproviderOnly creates a v17 repo with only Reprovider configuration
func setupV17RepoWithReproviderOnly(t *testing.T) *harness.Node {
	return setupV17RepoWithConfig(t,
		map[string]interface{}{},
		map[string]interface{}{
			"Strategy": "pinned",
			"Interval": "48h",
		})
}

// setupV17RepoWithInvalidStrategy creates a v17 repo with an invalid strategy value
func setupV17RepoWithInvalidStrategy(t *testing.T) *harness.Node {
	return setupV17RepoWithConfig(t,
		map[string]interface{}{},
		map[string]interface{}{
			"Strategy": "invalid-strategy", // This is not a valid strategy
			"Interval": "24h",
		})
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
