package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	. "github.com/ipfs/kubo/test/cli/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPDelegatedRouting(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init().StartDaemon()

	fakeServer := func(contentType string, resp ...string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", contentType)
			for _, r := range resp {
				_, err := w.Write([]byte(r))
				if err != nil {
					panic(err)
				}
			}
		}))
	}

	findProvsCID := "baeabep4vu3ceru7nerjjbk37sxb7wmftteve4hcosmyolsbsiubw2vr6pqzj6mw7kv6tbn6nqkkldnklbjgm5tzbi4hkpkled4xlcr7xz4bq"
	provs := []string{"12D3KooWAobjw92XDcnQ1rRmRJDA3zAQpdPYUpZKrJxH6yccSpje", "12D3KooWARYacCc6eoCqvsS9RW9MA2vo51CV75deoiqssx3YgyYJ"}

	t.Run("default routing config has no routers defined", func(t *testing.T) {
		assert.Nil(t, node.ReadConfig().Routing.Routers)
	})

	t.Run("no routers means findprovs returns no results", func(t *testing.T) {
		res := node.IPFS("routing", "findprovs", findProvsCID).Stdout.String()
		assert.Empty(t, res)
	})

	t.Run("no routers means findprovs returns no results", func(t *testing.T) {
		res := node.IPFS("routing", "findprovs", findProvsCID).Stdout.String()
		assert.Empty(t, res)
	})

	node.StopDaemon()

	t.Run("missing method params make the daemon fail", func(t *testing.T) {
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Routing.Type = config.NewOptionalString("custom")
			cfg.Routing.Methods = config.Methods{
				"find-peers":     {RouterName: "TestDelegatedRouter"},
				"find-providers": {RouterName: "TestDelegatedRouter"},
				"get-ipns":       {RouterName: "TestDelegatedRouter"},
				"provide":        {RouterName: "TestDelegatedRouter"},
			}
		})
		res := node.RunIPFS("daemon")
		assert.Equal(t, 1, res.ExitErr.ProcessState.ExitCode())
		assert.Contains(
			t,
			res.Stderr.String(),
			`method name "put-ipns" is missing from Routing.Methods config param`,
		)
	})

	t.Run("having wrong methods makes daemon fail", func(t *testing.T) {
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Routing.Type = config.NewOptionalString("custom")
			cfg.Routing.Methods = config.Methods{
				"find-peers":     {RouterName: "TestDelegatedRouter"},
				"find-providers": {RouterName: "TestDelegatedRouter"},
				"get-ipns":       {RouterName: "TestDelegatedRouter"},
				"provide":        {RouterName: "TestDelegatedRouter"},
				"put-ipns":       {RouterName: "TestDelegatedRouter"},
				"NOT_SUPPORTED":  {RouterName: "TestDelegatedRouter"},
			}
		})
		res := node.RunIPFS("daemon")
		assert.Equal(t, 1, res.ExitErr.ProcessState.ExitCode())
		assert.Contains(
			t,
			res.Stderr.String(),
			`method name "NOT_SUPPORTED" is not a supported method on Routing.Methods config param`,
		)
	})

	t.Run("adding HTTP delegated routing endpoint to Routing.Routers config works", func(t *testing.T) {
		server := fakeServer("application/json", ToJSONStr(JSONObj{
			"Providers": []JSONObj{
				{
					"Schema":   "bitswap", // Legacy bitswap schema.
					"Protocol": "transport-bitswap",
					"ID":       provs[1],
					"Addrs":    []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/tcp/4002"},
				},
				{
					"Schema":    "peer",
					"Protocols": []string{"transport-bitswap"},
					"ID":        provs[0],
					"Addrs":     []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/tcp/4002"},
				},
			},
		}))
		t.Cleanup(server.Close)

		node.IPFS("config", "Routing.Type", "custom")
		node.IPFS("config", "Routing.Routers.TestDelegatedRouter", "--json", ToJSONStr(JSONObj{
			"Type": "http",
			"Parameters": JSONObj{
				"Endpoint": server.URL,
			},
		}))
		node.IPFS("config", "Routing.Methods", "--json", ToJSONStr(JSONObj{
			"find-peers":     JSONObj{"RouterName": "TestDelegatedRouter"},
			"find-providers": JSONObj{"RouterName": "TestDelegatedRouter"},
			"get-ipns":       JSONObj{"RouterName": "TestDelegatedRouter"},
			"provide":        JSONObj{"RouterName": "TestDelegatedRouter"},
			"put-ipns":       JSONObj{"RouterName": "TestDelegatedRouter"},
		}))

		res := node.IPFS("config", "Routing.Routers.TestDelegatedRouter.Parameters.Endpoint")
		assert.Equal(t, res.Stdout.Trimmed(), server.URL)

		node.StartDaemon()
		res = node.IPFS("routing", "findprovs", findProvsCID)
		assert.Equal(t, provs[1]+"\n"+provs[0], res.Stdout.Trimmed())
	})

	node.StopDaemon()

	t.Run("adding HTTP delegated routing endpoint to Routing.Routers config works (streaming)", func(t *testing.T) {
		server := fakeServer("application/x-ndjson", ToJSONStr(JSONObj{
			"Schema":    "peer",
			"Protocols": []string{"transport-bitswap"},
			"ID":        provs[0],
			"Addrs":     []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/tcp/4002"},
		}), ToJSONStr(JSONObj{
			"Schema":   "bitswap", // Legacy bitswap schema.
			"Protocol": "transport-bitswap",
			"ID":       provs[1],
			"Addrs":    []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/tcp/4002"},
		}))
		t.Cleanup(server.Close)

		node.IPFS("config", "Routing.Routers.TestDelegatedRouter", "--json", ToJSONStr(JSONObj{
			"Type": "http",
			"Parameters": JSONObj{
				"Endpoint": server.URL,
			},
		}))

		res := node.IPFS("config", "Routing.Routers.TestDelegatedRouter.Parameters.Endpoint")
		assert.Equal(t, res.Stdout.Trimmed(), server.URL)

		node.StartDaemon()
		res = node.IPFS("routing", "findprovs", findProvsCID)
		assert.Equal(t, provs[0]+"\n"+provs[1], res.Stdout.Trimmed())
	})

	t.Run("HTTP client should emit OpenCensus metrics", func(t *testing.T) {
		resp := node.APIClient().Get("/debug/metrics/prometheus")
		assert.Contains(t, resp.Body, "routing_http_client_length_count")
	})
}

// TestHTTPDelegatedRoutingProviderAddrs verifies that provider records sent to
// HTTP routers contain the expected addresses based on Addresses configuration.
// See https://github.com/ipfs/kubo/issues/11213
func TestHTTPDelegatedRoutingProviderAddrs(t *testing.T) {
	t.Parallel()

	// captureProviderAddrs returns a mock server and a function to retrieve captured addresses.
	captureProviderAddrs := func(t *testing.T) (*httptest.Server, func() []string) {
		t.Helper()
		var mu sync.Mutex
		var capturedAddrs []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if (r.Method == http.MethodPut || r.Method == http.MethodPost) &&
				strings.HasPrefix(r.URL.Path, "/routing/v1/providers") {
				body, _ := io.ReadAll(r.Body)
				var envelope struct {
					Providers []struct {
						Payload json.RawMessage `json:"Payload"`
					} `json:"Providers"`
				}
				if json.Unmarshal(body, &envelope) == nil {
					for _, prov := range envelope.Providers {
						var payload struct {
							Addrs []string `json:"Addrs"`
						}
						if json.Unmarshal(prov.Payload, &payload) == nil && len(payload.Addrs) > 0 {
							mu.Lock()
							capturedAddrs = payload.Addrs
							mu.Unlock()
						}
					}
				}
				w.WriteHeader(http.StatusOK)
				return
			}
			if strings.HasPrefix(r.URL.Path, "/routing/v1/") {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		t.Cleanup(srv.Close)
		return srv, func() []string {
			mu.Lock()
			defer mu.Unlock()
			return capturedAddrs
		}
	}

	customRoutingConf := func(endpoint string) map[string]any {
		return map[string]any{
			"Type": "custom",
			"Methods": map[string]any{
				"provide":        map[string]any{"RouterName": "TestRouter"},
				"find-providers": map[string]any{"RouterName": "TestRouter"},
				"find-peers":     map[string]any{"RouterName": "TestRouter"},
				"get-ipns":       map[string]any{"RouterName": "TestRouter"},
				"put-ipns":       map[string]any{"RouterName": "TestRouter"},
			},
			"Routers": map[string]any{
				"TestRouter": map[string]any{
					"Type":       "http",
					"Parameters": map[string]any{"Endpoint": endpoint},
				},
			},
		}
	}

	t.Run("provider records respect user-provided Addresses.Announce override", func(t *testing.T) {
		t.Parallel()
		srv, getAddrs := captureProviderAddrs(t)

		node := harness.NewT(t).NewNode().Init()
		node.SetIPFSConfig("Addresses.Announce", []string{"/ip4/1.2.3.4/tcp/4001"})
		node.SetIPFSConfig("Routing", customRoutingConf(srv.URL))
		node.StartDaemon()
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(time.Now().String())
		node.IPFS("routing", "provide", cidStr)

		addrs := getAddrs()
		require.NotEmpty(t, addrs, "provider record should contain addresses")
		assert.Equal(t, []string{"/ip4/1.2.3.4/tcp/4001"}, addrs)
	})

	t.Run("provider records respect user-provided Addresses.AppendAnnounce", func(t *testing.T) {
		t.Parallel()
		srv, getAddrs := captureProviderAddrs(t)

		node := harness.NewT(t).NewNode().Init()
		node.SetIPFSConfig("Addresses.AppendAnnounce", []string{"/ip4/5.6.7.8/tcp/4001"})
		node.SetIPFSConfig("Routing", customRoutingConf(srv.URL))
		node.StartDaemon()
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(time.Now().String())
		node.IPFS("routing", "provide", cidStr)

		addrs := getAddrs()
		require.NotEmpty(t, addrs, "provider record should contain addresses")
		assert.Contains(t, addrs, "/ip4/5.6.7.8/tcp/4001", "AppendAnnounce address should be present")
	})
}
