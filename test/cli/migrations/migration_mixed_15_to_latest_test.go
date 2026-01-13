package migrations

// NOTE: These mixed migration tests validate the transition from old Kubo versions that used external
// migration binaries to the latest version with embedded migrations. This ensures users can upgrade
// from very old installations (v15) to the latest version seamlessly.
//
// The tests verify hybrid migration paths:
// - Forward: external binary (15→16) + embedded migrations (16→latest)
// - Backward: embedded migrations (latest→16) + external binary (16→15)
//
// This confirms compatibility between the old external migration system and the new embedded system.
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
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	ipfs "github.com/ipfs/kubo"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/require"
)

// TestMixedMigration15ToLatest tests migration from old Kubo (v15 with external migrations)
// to the latest version using a hybrid approach: external binary for 15→16, then embedded
// migrations for 16→latest. This ensures backward compatibility for users upgrading from
// very old Kubo installations.
func TestMixedMigration15ToLatest(t *testing.T) {
	t.Parallel()

	// Test mixed migration from v15 to latest (combines external 15→16 + embedded 16→latest)
	t.Run("daemon migrate: mixed 15 to latest", testDaemonMigration15ToLatest)
	t.Run("repo migrate: mixed 15 to latest", testRepoMigration15ToLatest)
}

// TestMixedMigrationLatestTo15Downgrade tests downgrading from the latest version back to v15
// using a hybrid approach: embedded migrations for latest→16, then external binary for 16→15.
// This ensures the migration system works bidirectionally for recovery scenarios.
func TestMixedMigrationLatestTo15Downgrade(t *testing.T) {
	t.Parallel()

	// Test reverse hybrid migration from latest to v15 (embedded latest→16 + external 16→15)
	t.Run("repo migrate: reverse hybrid latest to 15", testRepoReverseHybridMigrationLatestTo15)
}

func testDaemonMigration15ToLatest(t *testing.T) {
	// TEST: Migration from v15 to latest using 'ipfs daemon --migrate'
	// This tests the mixed migration path: external binary (15→16) + embedded (16→latest)
	node := setupStaticV15Repo(t)

	// Create mock migration binary for 15→16 (16→17 will use embedded migration)
	mockBinDir := createMockMigrationBinary(t, "15", "16")
	customPath := buildCustomPath(mockBinDir)

	configPath := filepath.Join(node.Dir, "config")
	versionPath := filepath.Join(node.Dir, "version")

	// Verify starting conditions
	versionData, err := os.ReadFile(versionPath)
	require.NoError(t, err)
	require.Equal(t, "15", strings.TrimSpace(string(versionData)), "Should start at version 15")

	// Read original config to verify preservation of key fields
	var originalConfig map[string]interface{}
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(configData, &originalConfig))

	originalPeerID := getNestedValue(originalConfig, "Identity.PeerID")

	// Run dual migration using daemon --migrate
	stdoutOutput, migrationSuccess := runDaemonWithLegacyMigrationMonitoring(t, node, customPath)

	// Debug output
	t.Logf("Daemon output:\n%s", stdoutOutput)

	// Verify hybrid migration was successful
	require.True(t, migrationSuccess, "Hybrid migration should have been successful")
	require.Contains(t, stdoutOutput, "Phase 1: External migration from v15 to v16", "Should detect external migration phase")
	// Verify each embedded migration step from 16 to latest
	verifyMigrationSteps(t, stdoutOutput, 16, ipfs.RepoVersion, true)
	require.Contains(t, stdoutOutput, fmt.Sprintf("Phase 2: Embedded migration from v16 to v%d", ipfs.RepoVersion), "Should detect embedded migration phase")
	require.Contains(t, stdoutOutput, "Hybrid migration completed successfully", "Should confirm hybrid migration completion")

	// Verify final version is latest
	versionData, err = os.ReadFile(versionPath)
	require.NoError(t, err)
	latestVersion := fmt.Sprintf("%d", ipfs.RepoVersion)
	require.Equal(t, latestVersion, strings.TrimSpace(string(versionData)), "Version should be updated to latest")

	// Verify config is still valid JSON and key fields preserved
	var finalConfig map[string]interface{}
	configData, err = os.ReadFile(configPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(configData, &finalConfig), "Config should remain valid JSON")

	// Verify essential fields preserved
	finalPeerID := getNestedValue(finalConfig, "Identity.PeerID")
	require.Equal(t, originalPeerID, finalPeerID, "Identity.PeerID should be preserved")

	// Verify bootstrap exists (may be modified by 16→17 migration)
	finalBootstrap := getNestedValue(finalConfig, "Bootstrap")
	require.NotNil(t, finalBootstrap, "Bootstrap should exist after migration")

	// Verify AutoConf was added by 16→17 migration
	autoConf := getNestedValue(finalConfig, "AutoConf")
	require.NotNil(t, autoConf, "AutoConf should be added by 16→17 migration")
}

func testRepoMigration15ToLatest(t *testing.T) {
	// TEST: Migration from v15 to latest using 'ipfs repo migrate'
	// Comparison test to verify repo migrate produces same results as daemon migrate
	node := setupStaticV15Repo(t)

	// Create mock migration binary for 15→16 (16→17 will use embedded migration)
	mockBinDir := createMockMigrationBinary(t, "15", "16")
	customPath := buildCustomPath(mockBinDir)

	configPath := filepath.Join(node.Dir, "config")
	versionPath := filepath.Join(node.Dir, "version")

	// Verify starting version
	versionData, err := os.ReadFile(versionPath)
	require.NoError(t, err)
	require.Equal(t, "15", strings.TrimSpace(string(versionData)), "Should start at version 15")

	// Run migration using 'ipfs repo migrate' with custom PATH
	result := runMigrationWithCustomPath(node, customPath, "repo", "migrate")
	require.Empty(t, result.Stderr.String(), "Migration should succeed without errors")

	// Verify final version is latest
	versionData, err = os.ReadFile(versionPath)
	require.NoError(t, err)
	latestVersion := fmt.Sprintf("%d", ipfs.RepoVersion)
	require.Equal(t, latestVersion, strings.TrimSpace(string(versionData)), "Version should be updated to latest")

	// Verify config is valid JSON
	var finalConfig map[string]interface{}
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(configData, &finalConfig), "Config should remain valid JSON")

	// Verify essential fields exist
	require.NotNil(t, getNestedValue(finalConfig, "Identity.PeerID"), "Identity.PeerID should exist")
	require.NotNil(t, getNestedValue(finalConfig, "Bootstrap"), "Bootstrap should exist")
	require.NotNil(t, getNestedValue(finalConfig, "AutoConf"), "AutoConf should be added")
}

// setupStaticV15Repo creates a test node using static v15 repo fixture
// This ensures tests remain stable and validates migration from very old repos
func setupStaticV15Repo(t *testing.T) *harness.Node {
	// Get path to static v15 repo fixture
	v15FixturePath := "testdata/v15-repo"

	// Create temporary test directory using Go's testing temp dir
	tmpDir := t.TempDir()

	// Use the built binary (should be in PATH)
	node := harness.BuildNode("ipfs", tmpDir, 0)

	// Copy static fixture to test directory
	cloneStaticRepoFixture(t, v15FixturePath, node.Dir)

	return node
}

// runDaemonWithLegacyMigrationMonitoring monitors for hybrid migration patterns
func runDaemonWithLegacyMigrationMonitoring(t *testing.T, node *harness.Node, customPath string) (string, bool) {
	// Monitor for hybrid migration completion - use "Hybrid migration completed successfully" as success pattern
	stdoutOutput, daemonStarted := runDaemonWithMigrationMonitoringCustomEnv(t, node, "Using hybrid migration strategy", "Hybrid migration completed successfully", map[string]string{
		"PATH": customPath, // Pass custom PATH with our mock binaries
	})

	// Check for hybrid migration patterns in output
	hasHybridStart := strings.Contains(stdoutOutput, "Using hybrid migration strategy")
	hasPhase1 := strings.Contains(stdoutOutput, "Phase 1: External migration from v15 to v16")
	hasPhase2 := strings.Contains(stdoutOutput, fmt.Sprintf("Phase 2: Embedded migration from v16 to v%d", ipfs.RepoVersion))
	hasHybridSuccess := strings.Contains(stdoutOutput, "Hybrid migration completed successfully")

	// Success requires daemon to start and hybrid migration patterns to be detected
	hybridMigrationSuccess := daemonStarted && hasHybridStart && hasPhase1 && hasPhase2 && hasHybridSuccess

	return stdoutOutput, hybridMigrationSuccess
}

// runDaemonWithMigrationMonitoringCustomEnv is like runDaemonWithMigrationMonitoring but allows custom environment
func runDaemonWithMigrationMonitoringCustomEnv(t *testing.T, node *harness.Node, migrationPattern, successPattern string, extraEnv map[string]string) (string, bool) {
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

	// Add extra environment variables (like PATH with mock binaries)
	for k, v := range extraEnv {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	// Set up pipes for output monitoring
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	stderr, err := cmd.StderrPipe()
	require.NoError(t, err)

	// Start the daemon
	require.NoError(t, cmd.Start())

	// Monitor output from both streams
	var outputBuffer strings.Builder
	done := make(chan bool)
	migrationStarted := false
	migrationCompleted := false

	go func() {
		scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
		for scanner.Scan() {
			line := scanner.Text()
			outputBuffer.WriteString(line + "\n")

			// Check for migration start
			if strings.Contains(line, migrationPattern) {
				migrationStarted = true
			}

			// Check for migration completion
			if strings.Contains(line, successPattern) {
				migrationCompleted = true
			}

			// Check for daemon ready
			if strings.Contains(line, "Daemon is ready") {
				done <- true
				return
			}
		}
		done <- false
	}()

	// Wait for daemon to be ready or timeout
	daemonReady := false
	select {
	case ready := <-done:
		daemonReady = ready
	case <-ctx.Done():
		t.Log("Daemon startup timed out")
	}

	// Stop the daemon using ipfs shutdown command for graceful shutdown
	if cmd.Process != nil {
		shutdownCmd := exec.Command(node.IPFSBin, "shutdown")
		shutdownCmd.Dir = node.Dir
		for k, v := range node.Runner.Env {
			shutdownCmd.Env = append(shutdownCmd.Env, k+"="+v)
		}

		if err := shutdownCmd.Run(); err != nil {
			// If graceful shutdown fails, force kill
			_ = cmd.Process.Kill()
		}

		// Wait for process to exit
		_ = cmd.Wait()
	}

	return outputBuffer.String(), daemonReady && migrationStarted && migrationCompleted
}

// buildCustomPath creates a custom PATH with mock migration binaries prepended.
// This is necessary for test isolation when running tests in parallel with t.Parallel().
// Without isolated PATH handling, parallel tests can interfere with each other through
// global PATH modifications, causing tests to download real migration binaries instead
// of using the test mocks.
func buildCustomPath(mockBinDirs ...string) string {
	// Prepend mock directories to ensure they're found first
	pathElements := append(mockBinDirs, os.Getenv("PATH"))
	return strings.Join(pathElements, string(filepath.ListSeparator))
}

// runMigrationWithCustomPath runs a migration command with a custom PATH environment.
// This ensures the migration uses our mock binaries instead of downloading real ones.
func runMigrationWithCustomPath(node *harness.Node, customPath string, args ...string) *harness.RunResult {
	return node.Runner.Run(harness.RunRequest{
		Path: node.IPFSBin,
		Args: args,
		CmdOpts: []harness.CmdOpt{
			func(cmd *exec.Cmd) {
				// Remove existing PATH entries using slices.DeleteFunc
				cmd.Env = slices.DeleteFunc(cmd.Env, func(s string) bool {
					return strings.HasPrefix(s, "PATH=")
				})
				// Add custom PATH
				cmd.Env = append(cmd.Env, "PATH="+customPath)
			},
		},
	})
}

// createMockMigrationBinary creates a platform-agnostic Go binary for migration testing.
// Returns the directory containing the binary to be added to PATH.
func createMockMigrationBinary(t *testing.T, fromVer, toVer string) string {
	// Create bin directory for migration binaries
	binDir := t.TempDir()

	// Create Go source for mock migration binary
	scriptName := fmt.Sprintf("fs-repo-%s-to-%s", fromVer, toVer)
	sourceFile := filepath.Join(binDir, scriptName+".go")
	binaryPath := filepath.Join(binDir, scriptName)
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	// Generate minimal mock migration binary code
	goSource := fmt.Sprintf(`package main
import ("fmt"; "os"; "path/filepath"; "strings"; "time")
func main() {
	var path string
	var revert bool
	for _, a := range os.Args[1:] {
		if strings.HasPrefix(a, "-path=") { path = a[6:] }
		if a == "-revert" { revert = true }
	}
	if path == "" { fmt.Fprintln(os.Stderr, "missing -path="); os.Exit(1) }

	from, to := "%s", "%s"
	if revert { from, to = to, from }
	fmt.Printf("fake applying %%s-to-%%s repo migration\n", from, to)

	// Create and immediately remove lock file to simulate proper locking behavior
	lockPath := filepath.Join(path, "repo.lock")
	lockFile, err := os.Create(lockPath)
	if err != nil && !os.IsExist(err) {
		fmt.Fprintf(os.Stderr, "Error creating lock: %%v\n", err)
		os.Exit(1)
	}
	if lockFile != nil {
		lockFile.Close()
		defer os.Remove(lockPath)
	}

	// Small delay to simulate migration work
	time.Sleep(10 * time.Millisecond)

	if err := os.WriteFile(filepath.Join(path, "version"), []byte(to), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %%v\n", err)
		os.Exit(1)
	}
}`, fromVer, toVer)

	require.NoError(t, os.WriteFile(sourceFile, []byte(goSource), 0644))

	// Compile the Go binary
	cmd := exec.Command("go", "build", "-o", binaryPath, sourceFile)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0") // Ensure static binary
	require.NoError(t, cmd.Run())

	// Verify the binary exists and is executable
	_, err := os.Stat(binaryPath)
	require.NoError(t, err, "Mock binary should exist")

	// Return the bin directory to be added to PATH
	return binDir
}

// expectedMigrationSteps generates the expected migration step strings for a version range.
// For forward migrations (from < to), it returns strings like "Running embedded migration fs-repo-16-to-17"
// For reverse migrations (from > to), it returns strings for the reverse path.
func expectedMigrationSteps(from, to int, forward bool) []string {
	var steps []string

	if forward {
		// Forward migration: increment by 1 each step
		for v := from; v < to; v++ {
			migrationName := fmt.Sprintf("fs-repo-%d-to-%d", v, v+1)
			steps = append(steps, fmt.Sprintf("Running embedded migration %s", migrationName))
		}
	} else {
		// Reverse migration: decrement by 1 each step
		for v := from; v > to; v-- {
			migrationName := fmt.Sprintf("fs-repo-%d-to-%d", v, v-1)
			steps = append(steps, fmt.Sprintf("Running reverse migration %s", migrationName))
		}
	}

	return steps
}

// verifyMigrationSteps checks that all expected migration steps appear in the output
func verifyMigrationSteps(t *testing.T, output string, from, to int, forward bool) {
	steps := expectedMigrationSteps(from, to, forward)
	for _, step := range steps {
		require.Contains(t, output, step, "Migration output should contain: %s", step)
	}
}

// getNestedValue retrieves a nested value from a config map using dot notation
func getNestedValue(config map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	current := interface{}(config)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			current = v[part]
		default:
			return nil
		}
		if current == nil {
			return nil
		}
	}

	return current
}

func testRepoReverseHybridMigrationLatestTo15(t *testing.T) {
	// TEST: Reverse hybrid migration from latest to v15 using 'ipfs repo migrate --to=15 --allow-downgrade'
	// This tests reverse hybrid migration: embedded (17→16) + external (16→15)

	// Start with v15 fixture and migrate forward to latest to create proper backup files
	node := setupStaticV15Repo(t)

	// Create mock migration binaries for both forward and reverse migrations
	mockBinDirs := []string{
		createMockMigrationBinary(t, "15", "16"), // for forward migration
		createMockMigrationBinary(t, "16", "15"), // for downgrade
	}
	customPath := buildCustomPath(mockBinDirs...)

	configPath := filepath.Join(node.Dir, "config")
	versionPath := filepath.Join(node.Dir, "version")

	// Step 1: Forward migration from v15 to latest to create backup files
	t.Logf("Step 1: Forward migration v15 → v%d", ipfs.RepoVersion)
	result := runMigrationWithCustomPath(node, customPath, "repo", "migrate")

	// Debug: print the output to see what happened
	t.Logf("Forward migration stdout:\n%s", result.Stdout.String())
	t.Logf("Forward migration stderr:\n%s", result.Stderr.String())

	require.Empty(t, result.Stderr.String(), "Forward migration should succeed without errors")

	// Verify we're at latest version after forward migration
	versionData, err := os.ReadFile(versionPath)
	require.NoError(t, err)
	latestVersion := fmt.Sprintf("%d", ipfs.RepoVersion)
	require.Equal(t, latestVersion, strings.TrimSpace(string(versionData)), "Should be at latest version after forward migration")

	// Read config after forward migration to use as baseline for downgrade
	var latestConfig map[string]interface{}
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(configData, &latestConfig))

	originalPeerID := getNestedValue(latestConfig, "Identity.PeerID")

	// Step 2: Reverse hybrid migration from latest to v15
	t.Logf("Step 2: Reverse hybrid migration v%d → v15", ipfs.RepoVersion)
	result = runMigrationWithCustomPath(node, customPath, "repo", "migrate", "--to=15", "--allow-downgrade")
	require.Empty(t, result.Stderr.String(), "Reverse hybrid migration should succeed without errors")

	// Debug output
	t.Logf("Downgrade migration output:\n%s", result.Stdout.String())

	// Verify final version is 15
	versionData, err = os.ReadFile(versionPath)
	require.NoError(t, err)
	require.Equal(t, "15", strings.TrimSpace(string(versionData)), "Version should be updated to 15")

	// Verify config is still valid JSON and key fields preserved
	var finalConfig map[string]interface{}
	configData, err = os.ReadFile(configPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(configData, &finalConfig), "Config should remain valid JSON")

	// Verify essential fields preserved
	finalPeerID := getNestedValue(finalConfig, "Identity.PeerID")
	require.Equal(t, originalPeerID, finalPeerID, "Identity.PeerID should be preserved")

	// Verify bootstrap exists (may be modified by migrations)
	finalBootstrap := getNestedValue(finalConfig, "Bootstrap")
	require.NotNil(t, finalBootstrap, "Bootstrap should exist after migration")

	// AutoConf should be removed by the downgrade (was added in 16→17)
	autoConf := getNestedValue(finalConfig, "AutoConf")
	require.Nil(t, autoConf, "AutoConf should be removed by downgrade to v15")
}
