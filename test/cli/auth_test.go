package cli

import (
	"testing"

	"github.com/ipfs/kubo/client/rpc/auth"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
)

func TestAuth(t *testing.T) {
	t.Parallel()

	makeAndStartProtectedNode := func(t *testing.T, authorizations map[string]*config.RPCAuthScope) *harness.Node {
		authorizations["test-node-starter"] = &config.RPCAuthScope{
			HTTPAuthSecret: "bearer:test-node-starter",
			AllowedPaths:   []string{"/api/v0"},
		}

		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.API.Authorizations = authorizations
		})
		node.StartDaemonWithAuthorization("Bearer test-node-starter")
		return node
	}

	t.Run("Follows Allowed Paths", func(t *testing.T) {
		t.Parallel()

		node := makeAndStartProtectedNode(t, map[string]*config.RPCAuthScope{
			"userA": {
				HTTPAuthSecret: "bearer:userAToken",
				AllowedPaths:   []string{"/api/v0/id"},
			},
		})

		apiClient := node.APIClient()
		apiClient.Client.Transport = auth.NewAuthorizedRoundTripper("Bearer userAToken", apiClient.Client.Transport)

		// Can access ID.
		resp := apiClient.Post("/api/v0/id", nil)
		assert.Equal(t, 200, resp.StatusCode)

		// But not Ping.
		resp = apiClient.Post("/api/v0/ping", nil)
		assert.Equal(t, 403, resp.StatusCode)

		node.StopDaemon()
	})

	t.Run("Generic Allowed Path Gives Full Access", func(t *testing.T) {
		t.Parallel()

		node := makeAndStartProtectedNode(t, map[string]*config.RPCAuthScope{
			"userA": {
				HTTPAuthSecret: "bearer:userAToken",
				AllowedPaths:   []string{"/api/v0"},
			},
		})

		apiClient := node.APIClient()
		apiClient.Client.Transport = auth.NewAuthorizedRoundTripper("Bearer userAToken", apiClient.Client.Transport)

		resp := apiClient.Post("/api/v0/id", nil)
		assert.Equal(t, 200, resp.StatusCode)

		node.StopDaemon()
	})
}
