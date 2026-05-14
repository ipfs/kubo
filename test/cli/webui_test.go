package cli

import (
	"net/http"
	"testing"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
)

func TestWebUI(t *testing.T) {
	t.Parallel()

	t.Run("NoFetch=true shows not available error", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Gateway.NoFetch = true
		})

		node.StartDaemon("--offline")

		apiClient := node.APIClient()
		resp := apiClient.Get("/webui/")

		// Should return 503 Service Unavailable when WebUI is not in local store
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

		// Check response contains helpful information
		body := resp.Body
		assert.Contains(t, body, "IPFS WebUI Not Available")
		assert.Contains(t, body, "Gateway.NoFetch=true")
		assert.Contains(t, body, "ipfs pin add")
		assert.Contains(t, body, "ipfs dag import")
		assert.Contains(t, body, "https://github.com/ipfs/ipfs-webui/releases")
	})

	t.Run("DeserializedResponses=false shows incompatible error", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Gateway.DeserializedResponses = config.False
		})

		node.StartDaemon()

		apiClient := node.APIClient()
		resp := apiClient.Get("/webui/")

		// Should return 503 Service Unavailable
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

		// Check response contains incompatibility message
		body := resp.Body
		assert.Contains(t, body, "IPFS WebUI Incompatible")
		assert.Contains(t, body, "Gateway.DeserializedResponses=false")
		assert.Contains(t, body, "WebUI requires deserializing IPFS responses")
		assert.Contains(t, body, "Gateway.DeserializedResponses=true")
	})

	t.Run("Both NoFetch=true and DeserializedResponses=false shows incompatible error", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Gateway.NoFetch = true
			cfg.Gateway.DeserializedResponses = config.False
		})

		node.StartDaemon("--offline")

		apiClient := node.APIClient()
		resp := apiClient.Get("/webui/")

		// Should return 503 Service Unavailable
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

		// DeserializedResponses=false takes priority
		body := resp.Body
		assert.Contains(t, body, "IPFS WebUI Incompatible")
		assert.Contains(t, body, "Gateway.DeserializedResponses=false")
		// Should NOT mention NoFetch since DeserializedResponses check comes first
		assert.NotContains(t, body, "NoFetch")
	})
}
