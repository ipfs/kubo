package cli

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/require"
)

// TestAddressFileReady verifies that when address files ($IPFS_PATH/api and
// $IPFS_PATH/gateway) are created, the corresponding HTTP servers are ready
// to accept connections immediately. This prevents race conditions for tools
// like systemd path units that start services when these files appear.
func TestAddressFileReady(t *testing.T) {
	t.Parallel()

	t.Run("api file", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		node := h.NewNode().Init()

		// Start daemon in background (don't use StartDaemon which waits for API)
		res := node.Runner.MustRun(harness.RunRequest{
			Path:    node.IPFSBin,
			Args:    []string{"daemon"},
			RunFunc: (*exec.Cmd).Start,
		})
		node.Daemon = res
		defer node.StopDaemon()

		// Poll for api file to appear
		apiFile := filepath.Join(node.Dir, "api")
		var fileExists bool
		for i := 0; i < 100; i++ {
			if _, err := os.Stat(apiFile); err == nil {
				fileExists = true
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		require.True(t, fileExists, "api file should be created")

		// Read the api file to get the address
		apiAddr, err := node.TryAPIAddr()
		require.NoError(t, err)

		// Extract IP and port from multiaddr
		ip, err := apiAddr.ValueForProtocol(4) // P_IP4
		require.NoError(t, err)
		port, err := apiAddr.ValueForProtocol(6) // P_TCP
		require.NoError(t, err)

		// Immediately try to use the API - should work on first attempt
		url := "http://" + ip + ":" + port + "/api/v0/id"
		resp, err := http.Post(url, "", nil)
		require.NoError(t, err, "RPC API should be ready immediately when api file exists")
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("gateway file", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		node := h.NewNode().Init()

		// Start daemon in background
		res := node.Runner.MustRun(harness.RunRequest{
			Path:    node.IPFSBin,
			Args:    []string{"daemon"},
			RunFunc: (*exec.Cmd).Start,
		})
		node.Daemon = res
		defer node.StopDaemon()

		// Poll for gateway file to appear
		gatewayFile := filepath.Join(node.Dir, "gateway")
		var fileExists bool
		for i := 0; i < 100; i++ {
			if _, err := os.Stat(gatewayFile); err == nil {
				fileExists = true
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		require.True(t, fileExists, "gateway file should be created")

		// Read the gateway file to get the URL (already includes http:// prefix)
		gatewayURL, err := os.ReadFile(gatewayFile)
		require.NoError(t, err)

		// Immediately try to use the Gateway - should work on first attempt
		url := strings.TrimSpace(string(gatewayURL)) + "/ipfs/bafkqaaa" // empty file CID
		resp, err := http.Get(url)
		require.NoError(t, err, "Gateway should be ready immediately when gateway file exists")
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
