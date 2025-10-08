package migrations

// NOTE: These concurrent migration tests require the local Kubo binary (built with 'make build') to be in PATH.
//
// To run these tests successfully:
//   export PATH="$(pwd)/cmd/ipfs:$PATH"
//   go test ./test/cli/migrations/

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const daemonStartupWait = 2 * time.Second

// TestConcurrentMigrations tests concurrent daemon --migrate attempts
func TestConcurrentMigrations(t *testing.T) {
	t.Parallel()

	t.Run("concurrent daemon migrations prevented by lock", testConcurrentDaemonMigrations)
}

func testConcurrentDaemonMigrations(t *testing.T) {
	node := setupStaticV16Repo(t)

	// Start first daemon --migrate in background (holds repo.lock)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	firstDaemon := setupDaemonCmd(ctx, node, "daemon", "--migrate")
	require.NoError(t, firstDaemon.Start())
	defer func() {
		// Shutdown first daemon
		shutdownCmd := setupDaemonCmd(context.Background(), node, "shutdown")
		_ = shutdownCmd.Run()
		_ = firstDaemon.Wait()
	}()

	// Wait for first daemon to start and acquire lock
	time.Sleep(daemonStartupWait)

	// Attempt second daemon --migrate (should fail due to lock)
	secondDaemon := setupDaemonCmd(context.Background(), node, "daemon", "--migrate")
	output, err := secondDaemon.CombinedOutput()
	t.Logf("Second daemon output: %s", output)

	// Should fail with lock error
	require.Error(t, err, "second daemon should fail when first daemon holds lock")
	require.Contains(t, string(output), "lock", "error should mention lock")

	assertNoTempFiles(t, node.Dir, "no temp files should be created when lock fails")
}
