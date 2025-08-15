package cli

import (
	"net/http"
	"testing"
	"time"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
)

// TestGatewayLimits tests the gateway request limiting and timeout features.
// These are basic integration tests that verify the configuration works.
// For comprehensive tests, see:
// - github.com/ipfs/boxo/gateway/middleware_retrieval_timeout_test.go
// - github.com/ipfs/boxo/gateway/middleware_ratelimit_test.go
func TestGatewayLimits(t *testing.T) {
	t.Parallel()

	t.Run("RetrievalTimeout", func(t *testing.T) {
		t.Parallel()

		// Create a node with a short retrieval timeout
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			// Set a 1 second timeout for retrieval
			cfg.Gateway.RetrievalTimeout = config.NewOptionalDuration(1 * time.Second)
		})
		node.StartDaemon()

		// Add content that can be retrieved quickly
		cid := node.IPFSAddStr("test content")

		client := node.GatewayClient()

		// Normal request should succeed (content is local)
		resp := client.Get("/ipfs/" + cid)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "test content", resp.Body)

		// Request for non-existent content should timeout
		// Using a CID that has no providers (generated with ipfs add -n)
		nonExistentCID := "bafkreif6lrhgz3fpiwypdk65qrqiey7svgpggruhbylrgv32l3izkqpsc4"

		// Create a client with longer timeout than the gateway's retrieval timeout
		// to ensure we get the gateway's 504 response
		clientWithTimeout := &harness.HTTPClient{
			Client: &http.Client{
				Timeout: 5 * time.Second,
			},
			BaseURL: client.BaseURL,
		}

		resp = clientWithTimeout.Get("/ipfs/" + nonExistentCID)
		assert.Equal(t, http.StatusGatewayTimeout, resp.StatusCode, "Expected 504 Gateway Timeout for stuck retrieval")
		assert.Contains(t, resp.Body, "Unable to retrieve content within timeout period")
	})

	t.Run("MaxConcurrentRequests", func(t *testing.T) {
		t.Parallel()

		// Create a node with a low concurrent request limit
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			// Allow only 1 concurrent request to make test deterministic
			cfg.Gateway.MaxConcurrentRequests = config.NewOptionalInteger(1)
			// Set retrieval timeout so blocking requests don't hang forever
			cfg.Gateway.RetrievalTimeout = config.NewOptionalDuration(2 * time.Second)
		})
		node.StartDaemon()

		// Add some content - use a non-existent CID that will block during retrieval
		// to ensure we can control timing
		blockingCID := "bafkreif6lrhgz3fpiwypdk65qrqiey7svgpggruhbylrgv32l3izkqpsc4"
		normalCID := node.IPFSAddStr("test content for concurrent request limiting")

		client := node.GatewayClient()

		// First, verify single request succeeds
		resp := client.Get("/ipfs/" + normalCID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Now test deterministic 429 response:
		// Start a blocking request that will occupy the single slot,
		// then make another request that MUST get 429

		blockingStarted := make(chan bool)
		blockingDone := make(chan bool)

		// Start a request that will block (searching for non-existent content)
		go func() {
			blockingStarted <- true
			// This will block until timeout looking for providers
			client.Get("/ipfs/" + blockingCID)
			blockingDone <- true
		}()

		// Wait for blocking request to start and occupy the slot
		<-blockingStarted
		time.Sleep(1 * time.Second) // Ensure it has acquired the semaphore

		// This request MUST get 429 because the slot is occupied
		resp = client.Get("/ipfs/" + normalCID + "?must-get-429=true")
		assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode, "Second request must get 429 when slot is occupied")

		// Verify 429 response headers
		retryAfter := resp.Headers.Get("Retry-After")
		assert.NotEmpty(t, retryAfter, "Retry-After header must be set on 429 response")
		assert.Equal(t, "60", retryAfter, "Retry-After must be 60 seconds")

		cacheControl := resp.Headers.Get("Cache-Control")
		assert.Equal(t, "no-store", cacheControl, "Cache-Control must be no-store on 429 response")

		assert.Contains(t, resp.Body, "Too many requests", "429 response must contain error message")

		// Clean up: wait for blocking request to timeout (it will timeout due to gateway retrieval timeout)
		select {
		case <-blockingDone:
			// Good, it completed
		case <-time.After(10 * time.Second):
			// Give it more time if needed
		}

		// Wait a bit more to ensure slot is fully released
		time.Sleep(1 * time.Second)

		// After blocking request completes, new request should succeed
		resp = client.Get("/ipfs/" + normalCID + "?after-limit-cleared=true")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Request must succeed after slot is freed")
	})
}
