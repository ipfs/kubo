package cli

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/stretchr/testify/require"
)

func TestDaemon(t *testing.T) {
	t.Parallel()

	t.Run("daemon starts if api is set to null", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.SetIPFSConfig("Addresses.API", nil)
		node.Runner.MustRun(harness.RunRequest{
			Path:    node.IPFSBin,
			Args:    []string{"daemon"},
			RunFunc: (*exec.Cmd).Start, // Start without waiting for completion.
		})

		node.StopDaemon()
	})

	t.Run("daemon shuts down gracefully with active operations", func(t *testing.T) {
		t.Parallel()

		// Start daemon with multiple components active via config
		node := harness.NewT(t).NewNode().Init()

		// Enable experimental features and pubsub via config
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Pubsub.Enabled = config.True          // Instead of --enable-pubsub-experiment
			cfg.Experimental.P2pHttpProxy = true      // Enable P2P HTTP proxy
			cfg.Experimental.GatewayOverLibp2p = true // Enable gateway over libp2p
		})

		node.StartDaemon("--enable-gc")

		// Start background operations to simulate real daemon workload:
		// 1. "ipfs add" simulates content onboarding/ingestion work
		// 2. Gateway request simulates content retrieval and gateway processing work

		// Background operation 1: Continuous add of random data to simulate onboarding
		addDone := make(chan struct{})
		go func() {
			defer close(addDone)

			// Start the add command asynchronously
			res := node.Runner.Run(harness.RunRequest{
				Path:    node.IPFSBin,
				Args:    []string{"add", "--progress=false", "-"},
				RunFunc: (*exec.Cmd).Start,
				CmdOpts: []harness.CmdOpt{
					harness.RunWithStdin(&infiniteReader{}),
				},
			})

			// Wait for command to finish (when daemon stops)
			if res.Cmd != nil {
				_ = res.Cmd.Wait() // Ignore error, expect command to be killed during shutdown
			}
		}()

		// Background operation 2: Gateway CAR request to simulate retrieval work
		gatewayDone := make(chan struct{})
		go func() {
			defer close(gatewayDone)

			// First add a file sized to ensure gateway request takes ~1 minute
			largeData := make([]byte, 512*1024) // 512KB of data
			_, _ = rand.Read(largeData)         // Always succeeds for crypto/rand
			testCID := node.IPFSAdd(bytes.NewReader(largeData))

			// Get gateway address from config
			cfg := node.ReadConfig()
			gatewayMaddr, err := multiaddr.NewMultiaddr(cfg.Addresses.Gateway[0])
			if err != nil {
				return
			}
			gatewayAddr, err := manet.ToNetAddr(gatewayMaddr)
			if err != nil {
				return
			}

			// Request CAR but slow reading to simulate heavy gateway load
			gatewayURL := fmt.Sprintf("http://%s/ipfs/%s?format=car", gatewayAddr, testCID)

			client := &http.Client{Timeout: 90 * time.Second}
			resp, err := client.Get(gatewayURL)
			if err == nil {
				defer resp.Body.Close()
				// Read response slowly: 512KB ÷ 1KB × 125ms = ~64 seconds (1+ minute) total
				// This ensures operation is still active when we shutdown at 2 seconds
				buf := make([]byte, 1024) // 1KB buffer
				for {
					if _, err := io.ReadFull(resp.Body, buf); err != nil {
						return
					}
					time.Sleep(125 * time.Millisecond) // 125ms delay = ~64s total for 512KB
				}
			}
		}()

		// Let operations run for 2 seconds to ensure they're active
		time.Sleep(2 * time.Second)

		// Trigger graceful shutdown
		shutdownStart := time.Now()
		node.StopDaemon()
		shutdownDuration := time.Since(shutdownStart)

		// Verify clean shutdown:
		// - Daemon should stop within reasonable time (not hang)
		require.Less(t, shutdownDuration, 10*time.Second, "daemon should shut down within 10 seconds")

		// Wait for background operations to complete (with timeout)
		select {
		case <-addDone:
			// Good, add operation terminated
		case <-time.After(5 * time.Second):
			t.Error("add operation did not terminate within 5 seconds after daemon shutdown")
		}

		select {
		case <-gatewayDone:
			// Good, gateway operation terminated
		case <-time.After(5 * time.Second):
			t.Error("gateway operation did not terminate within 5 seconds after daemon shutdown")
		}

		// Verify we can restart with same repo (no lock issues)
		node.StartDaemon()
		node.StopDaemon()
	})
}

// infiniteReader provides an infinite stream of random data
type infiniteReader struct{}

func (r *infiniteReader) Read(p []byte) (n int, err error) {
	_, _ = rand.Read(p)               // Always succeeds for crypto/rand
	time.Sleep(50 * time.Millisecond) // Rate limit to simulate steady stream
	return len(p), nil
}
