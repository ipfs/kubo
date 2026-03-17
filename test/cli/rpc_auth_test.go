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

	t.Run("Requests without Authorization header are rejected when auth is enabled", func(t *testing.T) {
		t.Parallel()

		node := makeAndStartProtectedNode(t, map[string]*config.RPCAuthScope{
			"userA": {
				AuthSecret:   "bearer:mytoken",
				AllowedPaths: []string{"/api/v0"},
			},
		})

		// Create client with NO auth
		apiClient := node.APIClient() // Uses http.DefaultClient with no auth headers

		// Should be denied without auth header
		resp := apiClient.Post("/api/v0/id", nil)
		assert.Equal(t, 403, resp.StatusCode)

		// Should contain denial message
		assert.Contains(t, resp.Body, rpcDeniedMsg)

		node.StopDaemon()
	})

	t.Run("Version endpoint is always accessible even with limited AllowedPaths", func(t *testing.T) {
		t.Parallel()

		node := makeAndStartProtectedNode(t, map[string]*config.RPCAuthScope{
			"userA": {
				AuthSecret:   "bearer:mytoken",
				AllowedPaths: []string{"/api/v0/id"}, // Only /id allowed
			},
		})

		apiClient := node.APIClient()
		apiClient.Client = &http.Client{
			Transport: auth.NewAuthorizedRoundTripper("Bearer mytoken", http.DefaultTransport),
		}

		// Can access /version even though not in AllowedPaths
		resp := apiClient.Post("/api/v0/version", nil)
		assert.Equal(t, 200, resp.StatusCode)

		node.StopDaemon()
	})

	t.Run("User cannot access API with another user's secret", func(t *testing.T) {
		t.Parallel()

		node := makeAndStartProtectedNode(t, map[string]*config.RPCAuthScope{
			"alice": {
				AuthSecret:   "bearer:alice-secret",
				AllowedPaths: []string{"/api/v0/id"},
			},
			"bob": {
				AuthSecret:   "bearer:bob-secret",
				AllowedPaths: []string{"/api/v0/config"},
			},
		})

		// Alice tries to use Bob's secret
		apiClient := node.APIClient()
		apiClient.Client = &http.Client{
			Transport: auth.NewAuthorizedRoundTripper("Bearer bob-secret", http.DefaultTransport),
		}

		// Bob's secret should work for Bob's paths
		resp := apiClient.Post("/api/v0/config/show", nil)
		assert.Equal(t, 200, resp.StatusCode)

		// But not for Alice's paths (Bob doesn't have access to /id)
		resp = apiClient.Post("/api/v0/id", nil)
		assert.Equal(t, 403, resp.StatusCode)

		node.StopDaemon()
	})

	t.Run("Empty AllowedPaths denies all access except version", func(t *testing.T) {
		t.Parallel()

		node := makeAndStartProtectedNode(t, map[string]*config.RPCAuthScope{
			"userA": {
				AuthSecret:   "bearer:mytoken",
				AllowedPaths: []string{}, // Empty!
			},
		})

		apiClient := node.APIClient()
		apiClient.Client = &http.Client{
			Transport: auth.NewAuthorizedRoundTripper("Bearer mytoken", http.DefaultTransport),
		}

		// Should deny everything
		resp := apiClient.Post("/api/v0/id", nil)
		assert.Equal(t, 403, resp.StatusCode)

		resp = apiClient.Post("/api/v0/config/show", nil)
		assert.Equal(t, 403, resp.StatusCode)

		// Except version
		resp = apiClient.Post("/api/v0/version", nil)
		assert.Equal(t, 200, resp.StatusCode)

		node.StopDaemon()
	})

	t.Run("CLI commands fail without --api-auth when auth is enabled", func(t *testing.T) {
		t.Parallel()

		node := makeAndStartProtectedNode(t, map[string]*config.RPCAuthScope{
			"userA": {
				AuthSecret:   "bearer:mytoken",
				AllowedPaths: []string{"/api/v0"},
			},
		})

		// Try to run command without --api-auth flag
		resp := node.RunIPFS("id") // No --api-auth flag
		require.Error(t, resp.Err)
		require.Contains(t, resp.Stderr.String(), rpcDeniedMsg)

		node.StopDaemon()
	})
}
