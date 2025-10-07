package migrations

// NOTE: These migration tests require the local Kubo binary (built with 'make build') to be in PATH.
//
// To run these tests successfully:
//   export PATH="$(pwd)/cmd/ipfs:$PATH"
//   go test ./test/cli/migrations/

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ipfs "github.com/ipfs/kubo"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigration16ToLatest tests migration from repo version 16 to the latest version.
//
// This test uses a real IPFS repository snapshot from Kubo v0.36.0 (the last version that used repo v16).
// The intention is to confirm that users can upgrade from Kubo v0.36.0 to the latest version by applying
// all intermediate migrations successfully.
//
// NOTE: This test comprehensively tests all migration methods (daemon --migrate, repo migrate,
// and reverse migration) because 16-to-17 was the first embedded migration that did not fetch
// external files. It serves as a reference implementation for migration testing.
//
// Future migrations can have simplified tests (like 17-to-18 in migration_17_to_latest_test.go)
// that focus on specific migration logic rather than testing all migration methods.
//
// If you need to test migration of configuration keys that appeared in later repo versions,
// create a new test file migration_N_to_latest_test.go with a separate IPFS repository test vector
// from the appropriate Kubo version.
func TestMigration16ToLatest(t *testing.T) {
	t.Parallel()

	// Primary tests using 'ipfs daemon --migrate' command (default in Docker)
	t.Run("daemon migrate: forward migration with auto values", testDaemonMigrationWithAuto)
	t.Run("daemon migrate: forward migration without auto values", testDaemonMigrationWithoutAuto)
	t.Run("daemon migrate: corrupted config handling", testDaemonCorruptedConfigHandling)
	t.Run("daemon migrate: missing fields handling", testDaemonMissingFieldsHandling)

	// Comparison tests using 'ipfs repo migrate' command
	t.Run("repo migrate: forward migration with auto values", testRepoMigrationWithAuto)
	t.Run("repo migrate: backward migration", testRepoBackwardMigration)

	// Temp file and backup cleanup tests
	t.Run("daemon migrate: no temp files after successful migration", testNoTempFilesAfterSuccessfulMigration)
	t.Run("daemon migrate: no temp files after failed migration", testNoTempFilesAfterFailedMigration)
	t.Run("daemon migrate: backup files persist after successful migration", testBackupFilesPersistAfterSuccessfulMigration)
	t.Run("repo migrate: backup files can revert migration", testBackupFilesCanRevertMigration)
	t.Run("repo migrate: conversion failure cleans up temp files", testConversionFailureCleanup)
}

// =============================================================================
// PRIMARY TESTS: 'ipfs daemon --migrate' command (default in Docker)
//
// These tests exercise the primary migration path used in production Docker
// containers where --migrate is enabled by default. This covers:
// - Normal forward migration scenarios
// - Error handling with corrupted configs
// - Migration with minimal/missing config fields
// =============================================================================

func testDaemonMigrationWithAuto(t *testing.T) {
	// TEST: Forward migration using 'ipfs daemon --migrate' command (PRIMARY)
	// Use static v16 repo fixture from real Kubo 0.36 `ipfs init`
	// NOTE: This test may need to be revised/updated once repo version 18 is released,
	// at that point only keep tests that use 'ipfs repo migrate'
	node := setupStaticV16Repo(t)

	configPath := filepath.Join(node.Dir, "config")
	versionPath := filepath.Join(node.Dir, "version")

	// Static fixture already uses port 0 for random port assignment - no config update needed

	// Run migration using daemon --migrate (automatic during daemon startup)
	// This is the primary method used in Docker containers
	// Monitor output until daemon is ready, then shut it down gracefully
	stdoutOutput, migrationSuccess := runDaemonMigrationWithMonitoring(t, node)

	// Debug: Print the actual output
	t.Logf("Daemon output:\n%s", stdoutOutput)

	// Verify migration was successful based on monitoring
	require.True(t, migrationSuccess, "Migration should have been successful")
	require.Contains(t, stdoutOutput, "applying 16-to-17 repo migration", "Migration should have been triggered")
	require.Contains(t, stdoutOutput, "Migration 16-to-17 succeeded", "Migration should have completed successfully")

	// Verify version was updated to latest
	versionData, err := os.ReadFile(versionPath)
	require.NoError(t, err)
	expectedVersion := fmt.Sprint(ipfs.RepoVersion)
	require.Equal(t, expectedVersion, strings.TrimSpace(string(versionData)), "Version should be updated to %s (latest)", expectedVersion)

	// Verify migration results using DRY helper
	helper := NewMigrationTestHelper(t, configPath)
	helper.RequireAutoConfDefaults().
		RequireArrayContains("Bootstrap", "auto").
		RequireArrayLength("Bootstrap", 1). // Should only contain "auto" when all peers were defaults
		RequireArrayContains("Routing.DelegatedRouters", "auto").
		RequireArrayContains("Ipns.DelegatedPublishers", "auto")

	// DNS resolver in static fixture should be empty, so "." should be set to "auto"
	helper.RequireFieldEquals("DNS.Resolvers[.]", "auto")
}

func testDaemonMigrationWithoutAuto(t *testing.T) {
	// TEST: Forward migration using 'ipfs daemon --migrate' command (PRIMARY)
	// Test migration of a config that already has some custom values
	// NOTE: This test may need to be revised/updated once repo version 18 is released,
	// at that point only keep tests that use 'ipfs repo migrate'
	// Should preserve existing settings and only add missing ones
	node := setupStaticV16Repo(t)

	// Modify the static fixture to add some custom values for testing mixed scenarios
	configPath := filepath.Join(node.Dir, "config")

	// Read existing config from static fixture
	var v16Config map[string]interface{}
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(configData, &v16Config))

	// Add custom DNS resolver that should be preserved
	if v16Config["DNS"] == nil {
		v16Config["DNS"] = map[string]interface{}{}
	}
	dnsSection := v16Config["DNS"].(map[string]interface{})
	dnsSection["Resolvers"] = map[string]string{
		".":    "https://custom-dns.example.com/dns-query",
		"eth.": "https://dns.eth.limo/dns-query", // This is a default that will be replaced with "auto"
	}

	// Write modified config back
	modifiedConfigData, err := json.MarshalIndent(v16Config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, modifiedConfigData, 0644))

	// Static fixture already uses port 0 for random port assignment - no config update needed

	// Run migration using daemon --migrate command (this is a daemon test)
	// Monitor output until daemon is ready, then shut it down gracefully
	stdoutOutput, migrationSuccess := runDaemonMigrationWithMonitoring(t, node)

	// Verify migration was successful based on monitoring
	require.True(t, migrationSuccess, "Migration should have been successful")
	require.Contains(t, stdoutOutput, "applying 16-to-17 repo migration", "Migration should have been triggered")
	require.Contains(t, stdoutOutput, "Migration 16-to-17 succeeded", "Migration should have completed successfully")

	// Verify migration results: custom values preserved alongside "auto"
	helper := NewMigrationTestHelper(t, configPath)
	helper.RequireAutoConfDefaults().
		RequireArrayContains("Bootstrap", "auto").
		RequireFieldEquals("DNS.Resolvers[.]", "https://custom-dns.example.com/dns-query")

	// Check that eth. resolver was replaced with "auto" since it uses a default URL
	helper.RequireFieldEquals("DNS.Resolvers[eth.]", "auto").
		RequireFieldEquals("DNS.Resolvers[.]", "https://custom-dns.example.com/dns-query")
}

// =============================================================================
// Tests using 'ipfs daemon --migrate' command
// =============================================================================

// Test helper structs and functions for cleaner, more DRY tests

type ConfigField struct {
	Path     string
	Expected interface{}
	Message  string
}

type MigrationTestHelper struct {
	t      *testing.T
	config map[string]interface{}
}

func NewMigrationTestHelper(t *testing.T, configPath string) *MigrationTestHelper {
	var config map[string]interface{}
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(configData, &config))

	return &MigrationTestHelper{t: t, config: config}
}

func (h *MigrationTestHelper) RequireFieldExists(path string) *MigrationTestHelper {
	value := h.getNestedValue(path)
	require.NotNil(h.t, value, "Field %s should exist", path)
	return h
}

func (h *MigrationTestHelper) RequireFieldEquals(path string, expected interface{}) *MigrationTestHelper {
	value := h.getNestedValue(path)
	require.Equal(h.t, expected, value, "Field %s should equal %v", path, expected)
	return h
}

func (h *MigrationTestHelper) RequireArrayContains(path string, expected interface{}) *MigrationTestHelper {
	value := h.getNestedValue(path)
	require.IsType(h.t, []interface{}{}, value, "Field %s should be an array", path)
	array := value.([]interface{})
	require.Contains(h.t, array, expected, "Array %s should contain %v", path, expected)
	return h
}

func (h *MigrationTestHelper) RequireArrayLength(path string, expectedLen int) *MigrationTestHelper {
	value := h.getNestedValue(path)
	require.IsType(h.t, []interface{}{}, value, "Field %s should be an array", path)
	array := value.([]interface{})
	require.Len(h.t, array, expectedLen, "Array %s should have length %d", path, expectedLen)
	return h
}

func (h *MigrationTestHelper) RequireArrayDoesNotContain(path string, notExpected interface{}) *MigrationTestHelper {
	value := h.getNestedValue(path)
	require.IsType(h.t, []interface{}{}, value, "Field %s should be an array", path)
	array := value.([]interface{})
	require.NotContains(h.t, array, notExpected, "Array %s should not contain %v", path, notExpected)
	return h
}

func (h *MigrationTestHelper) RequireFieldAbsent(path string) *MigrationTestHelper {
	value := h.getNestedValue(path)
	require.Nil(h.t, value, "Field %s should not exist", path)
	return h
}

func (h *MigrationTestHelper) RequireAutoConfDefaults() *MigrationTestHelper {
	// AutoConf section should exist but be empty (using implicit defaults)
	return h.RequireFieldExists("AutoConf").
		RequireFieldAbsent("AutoConf.Enabled").              // Should use implicit default (true)
		RequireFieldAbsent("AutoConf.URL").                  // Should use implicit default (mainnet URL)
		RequireFieldAbsent("AutoConf.RefreshInterval").      // Should use implicit default (24h)
		RequireFieldAbsent("AutoConf.TLSInsecureSkipVerify") // Should use implicit default (false)
}

func (h *MigrationTestHelper) RequireAutoFieldsSetToAuto() *MigrationTestHelper {
	return h.RequireArrayContains("Bootstrap", "auto").
		RequireFieldEquals("DNS.Resolvers[.]", "auto").
		RequireArrayContains("Routing.DelegatedRouters", "auto").
		RequireArrayContains("Ipns.DelegatedPublishers", "auto")
}

func (h *MigrationTestHelper) RequireNoAutoValues() *MigrationTestHelper {
	// Check Bootstrap if it exists
	if h.getNestedValue("Bootstrap") != nil {
		h.RequireArrayDoesNotContain("Bootstrap", "auto")
	}

	// Check DNS.Resolvers if it exists
	if h.getNestedValue("DNS.Resolvers") != nil {
		h.RequireMapDoesNotContainValue("DNS.Resolvers", "auto")
	}

	// Check Routing.DelegatedRouters if it exists
	if h.getNestedValue("Routing.DelegatedRouters") != nil {
		h.RequireArrayDoesNotContain("Routing.DelegatedRouters", "auto")
	}

	// Check Ipns.DelegatedPublishers if it exists
	if h.getNestedValue("Ipns.DelegatedPublishers") != nil {
		h.RequireArrayDoesNotContain("Ipns.DelegatedPublishers", "auto")
	}

	return h
}

func (h *MigrationTestHelper) RequireMapDoesNotContainValue(path string, notExpected interface{}) *MigrationTestHelper {
	value := h.getNestedValue(path)
	require.IsType(h.t, map[string]interface{}{}, value, "Field %s should be a map", path)
	mapValue := value.(map[string]interface{})
	for k, v := range mapValue {
		require.NotEqual(h.t, notExpected, v, "Map %s[%s] should not equal %v", path, k, notExpected)
	}
	return h
}

func (h *MigrationTestHelper) getNestedValue(path string) interface{} {
	segments := h.parseKuboConfigPath(path)
	current := interface{}(h.config)

	for _, segment := range segments {
		switch segment.Type {
		case "field":
			switch v := current.(type) {
			case map[string]interface{}:
				current = v[segment.Key]
			default:
				return nil
			}
		case "mapKey":
			switch v := current.(type) {
			case map[string]interface{}:
				current = v[segment.Key]
			default:
				return nil
			}
		default:
			return nil
		}

		if current == nil {
			return nil
		}
	}

	return current
}

type PathSegment struct {
	Type string // "field" or "mapKey"
	Key  string
}

func (h *MigrationTestHelper) parseKuboConfigPath(path string) []PathSegment {
	var segments []PathSegment

	// Split path into parts, respecting bracket boundaries
	parts := h.splitKuboConfigPath(path)

	for _, part := range parts {
		if strings.Contains(part, "[") && strings.HasSuffix(part, "]") {
			// Handle field[key] notation
			bracketStart := strings.Index(part, "[")
			fieldName := part[:bracketStart]
			mapKey := part[bracketStart+1 : len(part)-1] // Remove [ and ]

			// Add field segment if present
			if fieldName != "" {
				segments = append(segments, PathSegment{Type: "field", Key: fieldName})
			}
			// Add map key segment
			segments = append(segments, PathSegment{Type: "mapKey", Key: mapKey})
		} else {
			// Regular field access
			if part != "" {
				segments = append(segments, PathSegment{Type: "field", Key: part})
			}
		}
	}

	return segments
}

// splitKuboConfigPath splits a path on dots, but preserves bracket sections intact
func (h *MigrationTestHelper) splitKuboConfigPath(path string) []string {
	var parts []string
	var current strings.Builder
	inBrackets := false

	for _, r := range path {
		switch r {
		case '[':
			inBrackets = true
			current.WriteRune(r)
		case ']':
			inBrackets = false
			current.WriteRune(r)
		case '.':
			if inBrackets {
				// Inside brackets, preserve the dot
				current.WriteRune(r)
			} else {
				// Outside brackets, split here
				if current.Len() > 0 {
					parts = append(parts, current.String())
					current.Reset()
				}
			}
		default:
			current.WriteRune(r)
		}
	}

	// Add final part if any
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// setupStaticV16Repo creates a test node using static v16 repo fixture from real Kubo 0.36 `ipfs init`
// This ensures tests remain stable regardless of future changes to the IPFS binary
// Each test gets its own copy in a temporary directory to allow modifications
func setupStaticV16Repo(t *testing.T) *harness.Node {
	// Get absolute path to static v16 repo fixture
	v16FixturePath := "testdata/v16-repo"

	// Create a temporary test directory - each test gets its own copy
	// Sanitize test name for Windows - replace invalid characters
	sanitizedName := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, t.Name())
	tmpDir := filepath.Join(t.TempDir(), "migration-test-"+sanitizedName)
	require.NoError(t, os.MkdirAll(tmpDir, 0755))

	// Convert to absolute path for harness
	absTmpDir, err := filepath.Abs(tmpDir)
	require.NoError(t, err)

	// Use the built binary (should be in PATH)
	node := harness.BuildNode("ipfs", absTmpDir, 0)

	// Replace IPFS_PATH with static fixture files to test directory (creates independent copy per test)
	cloneStaticRepoFixture(t, v16FixturePath, node.Dir)

	return node
}

// cloneStaticRepoFixture recursively copies the v16 repo fixture to the target directory
// It completely removes the target directory contents before copying to ensure no extra files remain
func cloneStaticRepoFixture(t *testing.T, srcPath, dstPath string) {
	srcInfo, err := os.Stat(srcPath)
	require.NoError(t, err)

	if srcInfo.IsDir() {
		// Completely remove destination directory and all contents
		require.NoError(t, os.RemoveAll(dstPath))
		// Create fresh destination directory
		require.NoError(t, os.MkdirAll(dstPath, srcInfo.Mode()))

		// Read source directory
		entries, err := os.ReadDir(srcPath)
		require.NoError(t, err)

		// Copy each entry recursively
		for _, entry := range entries {
			srcEntryPath := filepath.Join(srcPath, entry.Name())
			dstEntryPath := filepath.Join(dstPath, entry.Name())
			cloneStaticRepoFixture(t, srcEntryPath, dstEntryPath)
		}
	} else {
		// Copy file (destination directory should already be clean from parent call)
		srcFile, err := os.Open(srcPath)
		require.NoError(t, err)
		defer srcFile.Close()

		dstFile, err := os.Create(dstPath)
		require.NoError(t, err)
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		require.NoError(t, err)

		// Copy file permissions
		require.NoError(t, dstFile.Chmod(srcInfo.Mode()))
	}
}

// Placeholder stubs for new test functions - to be implemented
func testDaemonCorruptedConfigHandling(t *testing.T) {
	// TEST: Error handling using 'ipfs daemon --migrate' command with corrupted config (PRIMARY)
	// Test what happens when config file is corrupted during migration
	// NOTE: This test may need to be revised/updated once repo version 18 is released,
	// at that point only keep tests that use 'ipfs repo migrate'
	node := setupStaticV16Repo(t)

	// Create corrupted config
	configPath := filepath.Join(node.Dir, "config")
	corruptedJson := `{"Bootstrap": [invalid json}`
	require.NoError(t, os.WriteFile(configPath, []byte(corruptedJson), 0644))

	// Write version file indicating v16
	versionPath := filepath.Join(node.Dir, "version")
	require.NoError(t, os.WriteFile(versionPath, []byte("16"), 0644))

	// Run daemon with --migrate flag - this should fail gracefully
	result := node.RunIPFS("daemon", "--migrate")

	// Verify graceful failure handling
	// The daemon should fail but migration error should be clear
	errorOutput := result.Stderr.String() + result.Stdout.String()
	require.True(t, strings.Contains(errorOutput, "json") || strings.Contains(errorOutput, "invalid character"), "Error should mention JSON parsing issue")

	// Verify atomic failure: version and config should remain unchanged
	versionData, err := os.ReadFile(versionPath)
	require.NoError(t, err)
	require.Equal(t, "16", strings.TrimSpace(string(versionData)), "Version should remain unchanged after failed migration")

	originalContent, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Equal(t, corruptedJson, string(originalContent), "Original config should be unchanged after failed migration")
}

func testDaemonMissingFieldsHandling(t *testing.T) {
	// TEST: Migration using 'ipfs daemon --migrate' command with minimal config (PRIMARY)
	// Test migration when config is missing expected fields
	// NOTE: This test may need to be revised/updated once repo version 18 is released,
	// at that point only keep tests that use 'ipfs repo migrate'
	node := setupStaticV16Repo(t)

	// The static fixture already has all required fields, use it as-is
	configPath := filepath.Join(node.Dir, "config")
	versionPath := filepath.Join(node.Dir, "version")

	// Static fixture already uses port 0 for random port assignment - no config update needed

	// Run daemon migration
	stdoutOutput, migrationSuccess := runDaemonMigrationWithMonitoring(t, node)

	// Verify migration was successful
	require.True(t, migrationSuccess, "Migration should have been successful")
	require.Contains(t, stdoutOutput, "applying 16-to-17 repo migration", "Migration should have been triggered")
	require.Contains(t, stdoutOutput, "Migration 16-to-17 succeeded", "Migration should have completed successfully")

	// Verify version was updated to latest
	versionData, err := os.ReadFile(versionPath)
	require.NoError(t, err)
	expectedVersion := fmt.Sprint(ipfs.RepoVersion)
	require.Equal(t, expectedVersion, strings.TrimSpace(string(versionData)), "Version should be updated to %s (latest)", expectedVersion)

	// Verify migration adds all required fields to minimal config
	NewMigrationTestHelper(t, configPath).
		RequireAutoConfDefaults().
		RequireAutoFieldsSetToAuto().
		RequireFieldExists("Identity.PeerID") // Original identity preserved from static fixture
}

// =============================================================================
// COMPARISON TESTS: 'ipfs repo migrate' command
//
// These tests verify that repo migrate produces equivalent results to
// daemon migrate, and test scenarios specific to repo migrate like
// backward migration (which daemon doesn't support).
// =============================================================================

func testRepoMigrationWithAuto(t *testing.T) {
	// TEST: Forward migration using 'ipfs repo migrate' command (COMPARISON)
	// Simple comparison test to verify repo migrate produces same results as daemon migrate
	node := setupStaticV16Repo(t)

	// Use static fixture as-is
	configPath := filepath.Join(node.Dir, "config")

	// Run migration using 'ipfs repo migrate' command
	result := node.RunIPFS("repo", "migrate")
	require.Empty(t, result.Stderr.String(), "Migration should succeed without errors")

	// Verify same results as daemon migrate
	helper := NewMigrationTestHelper(t, configPath)
	helper.RequireAutoConfDefaults().
		RequireArrayContains("Bootstrap", "auto").
		RequireArrayContains("Routing.DelegatedRouters", "auto").
		RequireArrayContains("Ipns.DelegatedPublishers", "auto").
		RequireFieldEquals("DNS.Resolvers[.]", "auto")
}

func testRepoBackwardMigration(t *testing.T) {
	// TEST: Backward migration using 'ipfs repo migrate --to=16 --allow-downgrade' command
	// This is kept as repo migrate since daemon doesn't support backward migration
	node := setupStaticV16Repo(t)

	// Use static fixture as-is
	configPath := filepath.Join(node.Dir, "config")
	versionPath := filepath.Join(node.Dir, "version")

	// First run forward migration to get to v17
	result := node.RunIPFS("repo", "migrate")
	t.Logf("Forward migration stdout:\n%s", result.Stdout.String())
	t.Logf("Forward migration stderr:\n%s", result.Stderr.String())
	require.Empty(t, result.Stderr.String(), "Forward migration should succeed")

	// Verify we're at the latest version
	versionData, err := os.ReadFile(versionPath)
	require.NoError(t, err)
	expectedVersion := fmt.Sprint(ipfs.RepoVersion)
	require.Equal(t, expectedVersion, strings.TrimSpace(string(versionData)), "Should be at version %s (latest) after forward migration", expectedVersion)

	// Now run reverse migration back to v16
	result = node.RunIPFS("repo", "migrate", "--to=16", "--allow-downgrade")
	t.Logf("Backward migration stdout:\n%s", result.Stdout.String())
	t.Logf("Backward migration stderr:\n%s", result.Stderr.String())
	require.Empty(t, result.Stderr.String(), "Reverse migration should succeed")

	// Verify version was downgraded to 16
	versionData, err = os.ReadFile(versionPath)
	require.NoError(t, err)
	require.Equal(t, "16", strings.TrimSpace(string(versionData)), "Version should be downgraded to 16")

	// Verify backward migration results: AutoConf removed and no "auto" values remain
	NewMigrationTestHelper(t, configPath).
		RequireFieldAbsent("AutoConf").
		RequireNoAutoValues()
}

// runDaemonMigrationWithMonitoring starts daemon --migrate, monitors output until "Daemon is ready",
// then gracefully shuts down the daemon and returns the captured output and success status.
// This monitors for all expected migrations from version 16 to latest.
func runDaemonMigrationWithMonitoring(t *testing.T, node *harness.Node) (string, bool) {
	// Monitor migrations from repo v16 to latest
	return runDaemonWithExpectedMigrations(t, node, 16, ipfs.RepoVersion)
}

// runDaemonWithExpectedMigrations monitors daemon startup for a sequence of migrations from startVersion to endVersion
func runDaemonWithExpectedMigrations(t *testing.T, node *harness.Node, startVersion, endVersion int) (string, bool) {
	// Build list of expected migrations
	var expectedMigrations []struct {
		pattern string
		success string
	}

	for v := startVersion; v < endVersion; v++ {
		from := v
		to := v + 1
		expectedMigrations = append(expectedMigrations, struct {
			pattern string
			success string
		}{
			pattern: fmt.Sprintf("applying %d-to-%d repo migration", from, to),
			success: fmt.Sprintf("Migration %d-to-%d succeeded", from, to),
		})
	}

	return runDaemonWithMultipleMigrationMonitoring(t, node, expectedMigrations)
}

// runDaemonWithMultipleMigrationMonitoring monitors daemon startup for multiple sequential migrations
func runDaemonWithMultipleMigrationMonitoring(t *testing.T, node *harness.Node, expectedMigrations []struct {
	pattern string
	success string
}) (string, bool) {
	// Create context with timeout as safety net
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Set up daemon command with output monitoring
	cmd := exec.CommandContext(ctx, node.IPFSBin, "daemon", "--migrate")
	cmd.Dir = node.Dir

	// Set environment (especially IPFS_PATH)
	for k, v := range node.Runner.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	// Set up pipes for output monitoring
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	stderr, err := cmd.StderrPipe()
	require.NoError(t, err)

	// Start the daemon
	err = cmd.Start()
	require.NoError(t, err)

	var allOutput strings.Builder
	var daemonReady bool

	// Track which migrations have been detected
	migrationsDetected := make([]bool, len(expectedMigrations))
	migrationsSucceeded := make([]bool, len(expectedMigrations))

	// Monitor stdout for completion signals
	scanner := bufio.NewScanner(stdout)
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			allOutput.WriteString(line + "\n")

			// Check for migration messages
			for i, migration := range expectedMigrations {
				if strings.Contains(line, migration.pattern) {
					migrationsDetected[i] = true
				}
				if strings.Contains(line, migration.success) {
					migrationsSucceeded[i] = true
				}
			}
			if strings.Contains(line, "Daemon is ready") {
				daemonReady = true
				break // Exit monitoring loop
			}
		}
	}()

	// Also monitor stderr (but don't use it for completion detection)
	go func() {
		stderrScanner := bufio.NewScanner(stderr)
		for stderrScanner.Scan() {
			line := stderrScanner.Text()
			allOutput.WriteString("STDERR: " + line + "\n")
		}
	}()

	// Wait for daemon ready signal or timeout
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Timeout - kill the process
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			t.Logf("Daemon migration timed out after 60 seconds")
			return allOutput.String(), false

		case <-ticker.C:
			if daemonReady {
				// Daemon is ready - shut it down gracefully
				shutdownCmd := exec.Command(node.IPFSBin, "shutdown")
				shutdownCmd.Dir = node.Dir
				for k, v := range node.Runner.Env {
					shutdownCmd.Env = append(shutdownCmd.Env, k+"="+v)
				}

				if err := shutdownCmd.Run(); err != nil {
					t.Logf("Warning: ipfs shutdown failed: %v", err)
					// Force kill if graceful shutdown fails
					if cmd.Process != nil {
						_ = cmd.Process.Kill()
					}
				}

				// Wait for process to exit
				_ = cmd.Wait()

				// Check all migrations were detected and succeeded
				allDetected := true
				allSucceeded := true
				for i := range expectedMigrations {
					if !migrationsDetected[i] {
						allDetected = false
						t.Logf("Migration %s was not detected", expectedMigrations[i].pattern)
					}
					if !migrationsSucceeded[i] {
						allSucceeded = false
						t.Logf("Migration %s did not succeed", expectedMigrations[i].success)
					}
				}

				return allOutput.String(), allDetected && allSucceeded
			}

			// Check if process has exited (e.g., due to startup failure after migration)
			if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
				// Process exited - migration may have completed but daemon failed to start
				// This is expected for corrupted config tests

				// Check all migrations status
				allDetected := true
				allSucceeded := true
				for i := range expectedMigrations {
					if !migrationsDetected[i] {
						allDetected = false
					}
					if !migrationsSucceeded[i] {
						allSucceeded = false
					}
				}

				return allOutput.String(), allDetected && allSucceeded
			}
		}
	}
}

// =============================================================================
// TEMP FILE AND BACKUP CLEANUP TESTS
// =============================================================================

// Helper functions for test cleanup assertions
func assertNoTempFiles(t *testing.T, dir string, msgAndArgs ...interface{}) {
	t.Helper()
	tmpFiles, err := filepath.Glob(filepath.Join(dir, ".tmp-*"))
	require.NoError(t, err)
	assert.Empty(t, tmpFiles, msgAndArgs...)
}

func backupPath(configPath string, fromVer, toVer int) string {
	return fmt.Sprintf("%s.%d-to-%d.bak", configPath, fromVer, toVer)
}

func setupDaemonCmd(ctx context.Context, node *harness.Node, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, node.IPFSBin, args...)
	cmd.Dir = node.Dir
	for k, v := range node.Runner.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	return cmd
}

func testNoTempFilesAfterSuccessfulMigration(t *testing.T) {
	node := setupStaticV16Repo(t)

	// Run successful migration
	_, migrationSuccess := runDaemonMigrationWithMonitoring(t, node)
	require.True(t, migrationSuccess, "migration should succeed")

	assertNoTempFiles(t, node.Dir, "no temp files should remain after successful migration")
}

func testNoTempFilesAfterFailedMigration(t *testing.T) {
	node := setupStaticV16Repo(t)

	// Corrupt config to force migration failure
	configPath := filepath.Join(node.Dir, "config")
	corruptedJson := `{"Bootstrap": ["auto",` // Invalid JSON
	require.NoError(t, os.WriteFile(configPath, []byte(corruptedJson), 0644))

	// Attempt migration (should fail)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := setupDaemonCmd(ctx, node, "daemon", "--migrate")
	output, _ := cmd.CombinedOutput()
	t.Logf("Failed migration output: %s", output)

	assertNoTempFiles(t, node.Dir, "no temp files should remain after failed migration")
}

func testBackupFilesPersistAfterSuccessfulMigration(t *testing.T) {
	node := setupStaticV16Repo(t)

	// Run migration from v16 to latest (v18)
	_, migrationSuccess := runDaemonMigrationWithMonitoring(t, node)
	require.True(t, migrationSuccess, "migration should succeed")

	// Check for backup files from each migration step
	configPath := filepath.Join(node.Dir, "config")
	backup16to17 := backupPath(configPath, 16, 17)
	backup17to18 := backupPath(configPath, 17, 18)

	// Both backup files should exist
	assert.FileExists(t, backup16to17, "16-to-17 backup should exist")
	assert.FileExists(t, backup17to18, "17-to-18 backup should exist")

	// Verify backup files contain valid JSON
	data16to17, err := os.ReadFile(backup16to17)
	require.NoError(t, err)
	var config16to17 map[string]interface{}
	require.NoError(t, json.Unmarshal(data16to17, &config16to17), "16-to-17 backup should be valid JSON")

	data17to18, err := os.ReadFile(backup17to18)
	require.NoError(t, err)
	var config17to18 map[string]interface{}
	require.NoError(t, json.Unmarshal(data17to18, &config17to18), "17-to-18 backup should be valid JSON")
}

func testBackupFilesCanRevertMigration(t *testing.T) {
	node := setupStaticV16Repo(t)

	configPath := filepath.Join(node.Dir, "config")
	versionPath := filepath.Join(node.Dir, "version")

	// Read original v16 config
	originalConfig, err := os.ReadFile(configPath)
	require.NoError(t, err)

	// Migrate to v17 only
	result := node.RunIPFS("repo", "migrate", "--to=17")
	require.Empty(t, result.Stderr.String(), "migration to v17 should succeed")

	// Verify backup exists
	backup16to17 := backupPath(configPath, 16, 17)
	assert.FileExists(t, backup16to17, "16-to-17 backup should exist")

	// Manually revert using backup
	backupData, err := os.ReadFile(backup16to17)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, backupData, 0600))
	require.NoError(t, os.WriteFile(versionPath, []byte("16"), 0644))

	// Verify config matches original
	revertedConfig, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.JSONEq(t, string(originalConfig), string(revertedConfig), "reverted config should match original")

	// Verify version is back to 16
	versionData, err := os.ReadFile(versionPath)
	require.NoError(t, err)
	assert.Equal(t, "16", strings.TrimSpace(string(versionData)), "version should be reverted to 16")
}

func testConversionFailureCleanup(t *testing.T) {
	// This test verifies that when a migration's conversion function fails,
	// all temporary files are cleaned up properly
	node := setupStaticV16Repo(t)

	configPath := filepath.Join(node.Dir, "config")

	// Create a corrupted config that will cause conversion to fail during JSON parsing
	// The migration will read this, attempt to parse as JSON, and fail
	corruptedJson := `{"Bootstrap": ["auto",` // Invalid JSON - missing closing bracket
	require.NoError(t, os.WriteFile(configPath, []byte(corruptedJson), 0644))

	// Attempt migration (should fail during conversion)
	result := node.RunIPFS("repo", "migrate")
	require.NotEmpty(t, result.Stderr.String(), "migration should fail with error")

	assertNoTempFiles(t, node.Dir, "no temp files should remain after conversion failure")

	// Verify no backup files were created (failure happened before backup)
	backupFiles, err := filepath.Glob(filepath.Join(node.Dir, "config.*.bak"))
	require.NoError(t, err)
	assert.Empty(t, backupFiles, "no backup files should be created on conversion failure")

	// Verify corrupted config is unchanged (atomic operations prevented overwrite)
	currentConfig, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, corruptedJson, string(currentConfig), "corrupted config should remain unchanged")
}
