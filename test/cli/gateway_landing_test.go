package cli

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGatewayLandingPage(t *testing.T) {
	t.Parallel()

	t.Run("default landing page is served when RootRedirect is not set", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
		client := node.GatewayClient()

		resp := client.Get("/")
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "text/html; charset=utf-8", resp.Headers.Get("Content-Type"))
		assert.Contains(t, resp.Body, "Welcome to Kubo!")
		assert.Contains(t, resp.Body, `name="robots" content="noindex"`)
		assert.Contains(t, resp.Body, "Gateway.RootRedirect")
		assert.Contains(t, resp.Body, "github.com/ipfs/kubo")
	})

	t.Run("landing page returns 404 for non-root paths", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
		client := node.GatewayClient()

		resp := client.Get("/nonexistent-path")
		assert.Equal(t, 404, resp.StatusCode)
	})

	t.Run("RootRedirect takes precedence over landing page", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		node := h.NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Gateway.RootRedirect = "/ipfs/bafkqaaa"
		})
		node.StartDaemon("--offline")
		client := node.GatewayClient().DisableRedirects()

		resp := client.Get("/")
		assert.Equal(t, 302, resp.StatusCode)
		assert.Equal(t, "/ipfs/bafkqaaa", resp.Headers.Get("Location"))
	})

	t.Run("landing page is also served on RPC API port", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
		client := node.APIClient()

		resp := client.Get("/")
		assert.Equal(t, 200, resp.StatusCode)
		assert.Contains(t, resp.Body, "Welcome to Kubo!")
	})

	t.Run("landing page includes abuse reporting section", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
		client := node.GatewayClient()

		resp := client.Get("/")
		require.Equal(t, 200, resp.StatusCode)
		assert.Contains(t, resp.Body, "Abuse Reports")
		assert.Contains(t, resp.Body, "whois.domaintools.com")
	})

	t.Run("landing page respects Gateway.HTTPHeaders", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		node := h.NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Gateway.HTTPHeaders = map[string][]string{
				"X-Custom-Header": {"test-value"},
			}
		})
		node.StartDaemon("--offline")
		client := node.GatewayClient()

		resp := client.Get("/")
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "test-value", resp.Headers.Get("X-Custom-Header"))
	})

	t.Run("gateway paths still work with landing page enabled", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
		cid := node.IPFSAddStr("test content")
		client := node.GatewayClient()

		// /ipfs/ path should work
		resp := client.Get("/ipfs/" + cid)
		assert.Equal(t, 200, resp.StatusCode)
		assert.True(t, strings.Contains(resp.Body, "test content"))
	})

	t.Run("landing page works on localhost (implicitly enabled subdomain gateway)", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")

		// Get the gateway URL and replace 127.0.0.1 with localhost.
		// localhost is an implicitly enabled subdomain gateway (see defaultKnownGateways
		// in gateway.go). The landing page must work as a fallback even when the
		// hostname handler intercepts requests for known gateways.
		gwURL := node.GatewayURL()
		u, err := url.Parse(gwURL)
		require.NoError(t, err)
		u.Host = "localhost:" + u.Port()

		client := &harness.HTTPClient{
			Client:  http.DefaultClient,
			BaseURL: u.String(),
		}

		resp := client.Get("/")
		assert.Equal(t, 200, resp.StatusCode)
		assert.Contains(t, resp.Body, "Welcome to Kubo!")
	})
}
