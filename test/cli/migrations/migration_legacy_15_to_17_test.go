package migrations

// NOTE: These legacy migration tests require the local Kubo binary (built with 'make build') to be in PATH.
// The tests migrate from repo version 15 to 17, which requires both external (15→16) and embedded (16→17) migrations.
// This validates the transition from legacy external binaries to modern embedded migrations.
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
	"syscall"
	"testing"
	"time"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/require"
)

func TestMigration15To17(t *testing.T) {
	t.Parallel()

	// Test legacy migration from v15 to v17 (combines external 15→16 + embedded 16→17)
	t.Run("daemon migrate: legacy 15 to 17", testDaemonMigration15To17)
	t.Run("repo migrate: legacy 15 to 17", testRepoMigration15To17)
}

func TestMigration17To15Downgrade(t *testing.T) {
	t.Parallel()

	// Test reverse hybrid migration from v17 to v15 (embedded 17→16 + external 16→15)
	t.Run("repo migrate: reverse hybrid 17 to 15", testRepoReverseHybridMigration17To15)
}

func testDaemonMigration15To17(t *testing.T) {
	// TEST: Migration from v15 to v17 using 'ipfs daemon --migrate'
	// This tests the dual migration path: external binary (15→16) + embedded (16→17)
	// NOTE: This test may need to be revised/updated once repo version 18 is released,
	// at that point only keep tests that use 'ipfs repo migrate'
	node := setupStaticV15Repo(t)

	// Create mock migration binary for 15→16 (16→17 will use embedded migration)
	createMockMigrationBinary(t, "15", "16")

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
	stdoutOutput, migrationSuccess := runDaemonWithLegacyMigrationMonitoring(t, node)

	// Debug output
	t.Logf("Daemon output:\n%s", stdoutOutput)

	// Verify hybrid migration was successful
	require.True(t, migrationSuccess, "Hybrid migration should have been successful")
	require.Contains(t, stdoutOutput, "Phase 1: External migration from v15 to v16", "Should detect external migration phase")
	require.Contains(t, stdoutOutput, "Phase 2: Embedded migration from v16 to v17", "Should detect embedded migration phase")
	require.Contains(t, stdoutOutput, "Hybrid migration completed successfully", "Should confirm hybrid migration completion")

	// Verify final version is 17
	versionData, err = os.ReadFile(versionPath)
	require.NoError(t, err)
	require.Equal(t, "17", strings.TrimSpace(string(versionData)), "Version should be updated to 17")

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

func testRepoMigration15To17(t *testing.T) {
	// TEST: Migration from v15 to v17 using 'ipfs repo migrate'
	// Comparison test to verify repo migrate produces same results as daemon migrate
	node := setupStaticV15Repo(t)

	// Create mock migration binary for 15→16 (16→17 will use embedded migration)
	createMockMigrationBinary(t, "15", "16")

	configPath := filepath.Join(node.Dir, "config")
	versionPath := filepath.Join(node.Dir, "version")

	// Verify starting version
	versionData, err := os.ReadFile(versionPath)
	require.NoError(t, err)
	require.Equal(t, "15", strings.TrimSpace(string(versionData)), "Should start at version 15")

	// Run migration using 'ipfs repo migrate' with custom PATH
	result := node.Runner.Run(harness.RunRequest{
		Path: node.IPFSBin,
		Args: []string{"repo", "migrate"},
		CmdOpts: []harness.CmdOpt{
			func(cmd *exec.Cmd) {
				// Ensure the command inherits our modified PATH with mock binaries
				cmd.Env = append(cmd.Env, "PATH="+os.Getenv("PATH"))
			},
		},
	})
	require.Empty(t, result.Stderr.String(), "Migration should succeed without errors")

	// Verify final version is 17
	versionData, err = os.ReadFile(versionPath)
	require.NoError(t, err)
	require.Equal(t, "17", strings.TrimSpace(string(versionData)), "Version should be updated to 17")

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
func runDaemonWithLegacyMigrationMonitoring(t *testing.T, node *harness.Node) (string, bool) {
	// Monitor for hybrid migration completion - use "Hybrid migration completed successfully" as success pattern
	stdoutOutput, daemonStarted := runDaemonWithMigrationMonitoringCustomEnv(t, node, "Using hybrid migration strategy", "Hybrid migration completed successfully", map[string]string{
		"PATH": os.Getenv("PATH"), // Pass current PATH which includes our mock binaries
	})

	// Check for hybrid migration patterns in output
	hasHybridStart := strings.Contains(stdoutOutput, "Using hybrid migration strategy")
	hasPhase1 := strings.Contains(stdoutOutput, "Phase 1: External migration from v15 to v16")
	hasPhase2 := strings.Contains(stdoutOutput, "Phase 2: Embedded migration from v16 to v17")
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

	// Stop the daemon
	if cmd.Process != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		_ = cmd.Wait()
	}

	return outputBuffer.String(), daemonReady && migrationStarted && migrationCompleted
}

// createMockMigrationBinary creates a platform-agnostic Go binary for migration on PATH
func createMockMigrationBinary(t *testing.T, fromVer, toVer string) {
	// Create bin directory for migration binaries
	binDir := t.TempDir()

	// Create Go source for mock migration binary
	scriptName := fmt.Sprintf("fs-repo-%s-to-%s", fromVer, toVer)
	sourceFile := filepath.Join(binDir, scriptName+".go")
	binaryPath := filepath.Join(binDir, scriptName)

	goSource := fmt.Sprintf(`package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	// Parse command line arguments - real migration binaries expect -path=<repo-path>
	var repoPath string
	var revert bool
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "-path=") {
			repoPath = strings.TrimPrefix(arg, "-path=")
		} else if arg == "-revert" {
			revert = true
		}
	}
	
	if repoPath == "" {
		fmt.Fprintf(os.Stderr, "Usage: %%s -path=<repo-path> [-verbose=true] [-revert]\n", os.Args[0])
		os.Exit(1)
	}
	
	// Determine source and target versions based on revert flag
	var sourceVer, targetVer string
	if revert {
		// When reverting, we go backwards: fs-repo-15-to-16 with -revert goes 16→15
		sourceVer = "%s"
		targetVer = "%s"
	} else {
		// Normal forward migration: fs-repo-15-to-16 goes 15→16
		sourceVer = "%s"
		targetVer = "%s"
	}
	
	// Print migration message (same format as real migrations)
	fmt.Printf("fake applying %%s-to-%%s repo migration\n", sourceVer, targetVer)
	
	// Update version file
	versionFile := filepath.Join(repoPath, "version")
	err := os.WriteFile(versionFile, []byte(targetVer), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error updating version: %%v\n", err)
		os.Exit(1)
	}
}
`, toVer, fromVer, fromVer, toVer)

	require.NoError(t, os.WriteFile(sourceFile, []byte(goSource), 0644))

	// Compile the Go binary
	require.NoError(t, os.Setenv("CGO_ENABLED", "0")) // Ensure static binary
	require.NoError(t, exec.Command("go", "build", "-o", binaryPath, sourceFile).Run())

	// Add bin directory to PATH for this test
	currentPath := os.Getenv("PATH")
	newPath := binDir + string(filepath.ListSeparator) + currentPath
	require.NoError(t, os.Setenv("PATH", newPath))
	t.Cleanup(func() { os.Setenv("PATH", currentPath) })

	// Verify the binary exists and is executable
	_, err := os.Stat(binaryPath)
	require.NoError(t, err, "Mock binary should exist")
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

func testRepoReverseHybridMigration17To15(t *testing.T) {
	// TEST: Reverse hybrid migration from v17 to v15 using 'ipfs repo migrate --to=15 --allow-downgrade'
	// This tests reverse hybrid migration: embedded (17→16) + external (16→15)

	// Start with v15 fixture and migrate forward to v17 to create proper backup files
	node := setupStaticV15Repo(t)

	// Create mock migration binary for 15→16 (needed for forward migration)
	createMockMigrationBinary(t, "15", "16")
	// Create mock migration binary for 16→15 (needed for downgrade)
	createMockMigrationBinary(t, "16", "15")

	configPath := filepath.Join(node.Dir, "config")
	versionPath := filepath.Join(node.Dir, "version")

	// Step 1: Forward migration from v15 to v17 to create backup files
	t.Log("Step 1: Forward migration v15 → v17")
	result := node.Runner.Run(harness.RunRequest{
		Path: node.IPFSBin,
		Args: []string{"repo", "migrate"},
		CmdOpts: []harness.CmdOpt{
			func(cmd *exec.Cmd) {
				// Ensure the command inherits our modified PATH with mock binaries
				cmd.Env = append(cmd.Env, "PATH="+os.Getenv("PATH"))
			},
		},
	})

	// Debug: print the output to see what happened
	t.Logf("Forward migration stdout:\n%s", result.Stdout.String())
	t.Logf("Forward migration stderr:\n%s", result.Stderr.String())

	require.Empty(t, result.Stderr.String(), "Forward migration should succeed without errors")

	// Verify we're at v17 after forward migration
	versionData, err := os.ReadFile(versionPath)
	require.NoError(t, err)
	require.Equal(t, "17", strings.TrimSpace(string(versionData)), "Should be at version 17 after forward migration")

	// Read config after forward migration to use as baseline for downgrade
	var v17Config map[string]interface{}
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(configData, &v17Config))

	originalPeerID := getNestedValue(v17Config, "Identity.PeerID")

	// Step 2: Reverse hybrid migration from v17 to v15
	t.Log("Step 2: Reverse hybrid migration v17 → v15")
	result = node.Runner.Run(harness.RunRequest{
		Path: node.IPFSBin,
		Args: []string{"repo", "migrate", "--to=15", "--allow-downgrade"},
		CmdOpts: []harness.CmdOpt{
			func(cmd *exec.Cmd) {
				// Ensure the command inherits our modified PATH with mock binaries
				cmd.Env = append(cmd.Env, "PATH="+os.Getenv("PATH"))
			},
		},
	})
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
