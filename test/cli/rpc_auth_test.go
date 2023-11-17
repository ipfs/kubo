package cli

import (
	"net/http"
	"testing"

	"github.com/ipfs/kubo/client/rpc/auth"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const rpcDeniedMsg = "Kubo RPC Access Denied: Please provide a valid authorization token as defined in the API.Authorizations configuration."

func TestRPCAuth(t *testing.T) {
	t.Parallel()

	makeAndStartProtectedNode := func(t *testing.T, authorizations map[string]*config.RPCAuthScope) *harness.Node {
		authorizations["test-node-starter"] = &config.RPCAuthScope{
			AuthSecret:   "bearer:test-node-starter",
			AllowedPaths: []string{"/api/v0"},
		}

		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.API.Authorizations = authorizations
		})
		node.StartDaemonWithAuthorization("Bearer test-node-starter")
		return node
	}

	makeHTTPTest := func(authSecret, header string) func(t *testing.T) {
		return func(t *testing.T) {
			t.Parallel()
			t.Log(authSecret, header)

			node := makeAndStartProtectedNode(t, map[string]*config.RPCAuthScope{
				"userA": {
					AuthSecret:   authSecret,
					AllowedPaths: []string{"/api/v0/id"},
				},
			})

			apiClient := node.APIClient()
			apiClient.Client = &http.Client{
				Transport: auth.NewAuthorizedRoundTripper(header, http.DefaultTransport),
			}

			// Can access /id with valid token
			resp := apiClient.Post("/api/v0/id", nil)
			assert.Equal(t, 200, resp.StatusCode)

			// But not /config/show
			resp = apiClient.Post("/api/v0/config/show", nil)
			assert.Equal(t, 403, resp.StatusCode)

			// create client which sends invalid access token
			invalidApiClient := node.APIClient()
			invalidApiClient.Client = &http.Client{
				Transport: auth.NewAuthorizedRoundTripper("Bearer invalid", http.DefaultTransport),
			}

			// Can't access /id with invalid token
			errResp := invalidApiClient.Post("/api/v0/id", nil)
			assert.Equal(t, 403, errResp.StatusCode)

			node.StopDaemon()
		}
	}

	makeCLITest := func(authSecret string) func(t *testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			node := makeAndStartProtectedNode(t, map[string]*config.RPCAuthScope{
				"userA": {
					AuthSecret:   authSecret,
					AllowedPaths: []string{"/api/v0/id"},
				},
			})

			// Can access 'ipfs id'
			resp := node.RunIPFS("id", "--api-auth", authSecret)
			require.NoError(t, resp.Err)

			// But not 'ipfs config show'
			resp = node.RunIPFS("config", "show", "--api-auth", authSecret)
			require.Error(t, resp.Err)
			require.Contains(t, resp.Stderr.String(), rpcDeniedMsg)

			node.StopDaemon()
		}
	}

	for _, testCase := range []struct {
		name       string
		authSecret string
		header     string
	}{
		{"Bearer (no type)", "myToken", "Bearer myToken"},
		{"Bearer", "bearer:myToken", "Bearer myToken"},
		{"Basic (user:pass)", "basic:user:pass", "Basic dXNlcjpwYXNz"},
		{"Basic (encoded)", "basic:dXNlcjpwYXNz", "Basic dXNlcjpwYXNz"},
	} {
		t.Run("AllowedPaths on CLI "+testCase.name, makeCLITest(testCase.authSecret))
		t.Run("AllowedPaths on HTTP "+testCase.name, makeHTTPTest(testCase.authSecret, testCase.header))
	}

	t.Run("AllowedPaths set to /api/v0 Gives Full Access", func(t *testing.T) {
		t.Parallel()

		node := makeAndStartProtectedNode(t, map[string]*config.RPCAuthScope{
			"userA": {
				AuthSecret:   "bearer:userAToken",
				AllowedPaths: []string{"/api/v0"},
			},
		})

		apiClient := node.APIClient()
		apiClient.Client = &http.Client{
			Transport: auth.NewAuthorizedRoundTripper("Bearer userAToken", http.DefaultTransport),
		}

		resp := apiClient.Post("/api/v0/id", nil)
		assert.Equal(t, 200, resp.StatusCode)

		node.StopDaemon()
	})

	t.Run("API.Authorizations set to nil disables Authorization header check", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.API.Authorizations = nil
		})
		node.StartDaemon()

		apiClient := node.APIClient()
		resp := apiClient.Post("/api/v0/id", nil)
		assert.Equal(t, 200, resp.StatusCode)

		node.StopDaemon()
	})

	t.Run("API.Authorizations set to empty map disables Authorization header check", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.API.Authorizations = map[string]*config.RPCAuthScope{}
		})
		node.StartDaemon()

		apiClient := node.APIClient()
		resp := apiClient.Post("/api/v0/id", nil)
		assert.Equal(t, 200, resp.StatusCode)

		node.StopDaemon()
	})
}
