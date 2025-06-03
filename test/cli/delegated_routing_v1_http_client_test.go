package cli

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	. "github.com/ipfs/kubo/test/cli/testutils"
	"github.com/stretchr/testify/assert"
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
