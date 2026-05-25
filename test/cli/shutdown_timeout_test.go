package cli

import (
	"testing"
	"time"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/require"
)

const (
	// testShutdownTimeout overrides DefaultShutdownTimeout so the test
	// runs in seconds rather than the production default.
	testShutdownTimeout = 10 * time.Second
	// testShutdownCompletionBound is a soft upper bound for StopDaemon in
	// this test. StopDaemon escalates SIGTERM, SIGTERM, SIGQUIT, SIGKILL
	// itself (see harness/node.go), so anything close to this bound
	// indicates kubo's own bounded-shutdown logic failed.
	testShutdownCompletionBound = testShutdownTimeout + 5*time.Second
)

// TestShutdownTimeoutHonored exercises the bounded-shutdown logic end-to-end
// for the common case (no hung subsystems): the daemon must shut down
// cleanly well within the configured ShutdownTimeout, and pinned/MFS data
// must survive across the restart.
func TestShutdownTimeoutHonored(t *testing.T) {
	t.Parallel()
	h := harness.NewT(t)
	node := h.NewNode().Init()
	node.UpdateConfig(func(cfg *config.Config) {
		cfg.Internal.ShutdownTimeout = config.NewOptionalDuration(testShutdownTimeout)
	})
	node.StartDaemon()

	// Real data-path work that must survive shutdown.
	addCID := node.PipeStrToIPFS("survives shutdown", "add", "-q").Stdout.Trimmed()
	node.IPFS("files", "mkdir", "/persisted")

	// "diag healthy" must succeed while the daemon is running normally.
	require.Equal(t, 0, node.RunIPFS("diag", "healthy").ExitCode(),
		"diag healthy should succeed before shutdown is initiated")

	start := time.Now()
	node.StopDaemon()
	require.Less(t, time.Since(start), testShutdownCompletionBound,
		"graceful shutdown should complete well within the configured ShutdownTimeout")

	// Restart and verify data survived.
	node.StartDaemon()
	require.Contains(t, node.IPFS("pin", "ls").Stdout.String(), addCID,
		"pinned CID should survive shutdown+restart")
	require.Contains(t, node.IPFS("files", "ls").Stdout.String(), "persisted",
		"MFS content should survive shutdown+restart")
}

// TestShutdownTimeoutDisabled verifies that ShutdownTimeout=0 opts out of
// the bounded-shutdown logic and behaves like legacy kubo (no watchdog,
// no app.Stop deadline). The daemon must still shut down cleanly because
// no subsystem is actually hung.
func TestShutdownTimeoutDisabled(t *testing.T) {
	t.Parallel()
	h := harness.NewT(t)
	node := h.NewNode().Init()
	node.UpdateConfig(func(cfg *config.Config) {
		cfg.Internal.ShutdownTimeout = config.NewOptionalDuration(0)
	})
	node.StartDaemon()

	start := time.Now()
	node.StopDaemon()
	require.Less(t, time.Since(start), testShutdownCompletionBound,
		"graceful shutdown should still complete in reasonable time with ShutdownTimeout=0")
}
