package cli

import (
	"net/http"
	"strings"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDaemonGatewayUpWhenMountFails is a regression test for
// https://github.com/ipfs/kubo/issues/8977: when --mount is requested but the
// FUSE mount fails, the daemon used to abort before starting the HTTP gateway,
// so the node silently stopped acting as a gateway. The daemon must now warn
// and continue, keeping the gateway (and the rest of the daemon) up.
func TestDaemonGatewayUpWhenMountFails(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init()

	// Force a FUSE mount failure without requiring FUSE support on the host:
	// --mount-ipfs points at a path that does not exist, so mountFuse fails at
	// checkFusePath before any FUSE syscall. --mount is rejected in offline
	// mode, so the daemon runs online.
	node.StartDaemon(
		"--mount",
		"--mount-ipfs=/ipfs-kubo-mount-does-not-exist-8977",
		"--mount-ipns=/ipns-kubo-mount-does-not-exist-8977",
		"--mount-mfs=/mfs-kubo-mount-does-not-exist-8977",
	)
	t.Cleanup(func() { node.StopDaemon() })

	// The daemon stayed up: StartDaemon's WaitOnAPI already returned, and the
	// gateway address file was written by serveHTTPGateway.
	cid := node.IPFSAddStr("hello from a daemon whose mount failed")

	client := node.GatewayClient()
	client.TemplateData = map[string]string{"CID": cid}

	t.Run("gateway still serves content", func(t *testing.T) {
		resp := client.Get("/ipfs/{{.CID}}")
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		require.NotNil(t, resp.Body)
		assert.Equal(t, "hello from a daemon whose mount failed", resp.Body)
	})

	t.Run("mount failure was reported on stderr", func(t *testing.T) {
		stderr := node.Daemon.Stderr.String()
		assert.Contains(t, stderr, "--mount failed")
		assert.Contains(t, stderr, "continuing without FUSE mounts")
		// Sanity check: we did not silently mount anything.
		assert.False(t, strings.Contains(stderr, "IPFS mounted at:"))
	})
}
