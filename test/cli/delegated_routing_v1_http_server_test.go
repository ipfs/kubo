package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/boxo/routing/http/client"
	"github.com/ipfs/boxo/routing/http/types"
	"github.com/ipfs/boxo/routing/http/types/iter"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoutingV1Server(t *testing.T) {
	t.Parallel()

	setupNodes := func(t *testing.T) harness.Nodes {
		nodes := harness.NewT(t).NewNodes(5).Init()
		nodes.ForEachPar(func(node *harness.Node) {
			node.UpdateConfig(func(cfg *config.Config) {
				cfg.Gateway.ExposeRoutingAPI = config.True
				cfg.Routing.Type = config.NewOptionalString("dht")
			})
		})
		nodes.StartDaemons().Connect()
		t.Cleanup(func() { nodes.StopDaemons() })
		return nodes
	}

	t.Run("Get Providers Responds With Correct Peers", func(t *testing.T) {
		t.Parallel()
		nodes := setupNodes(t)

		text := "hello world " + uuid.New().String()
		cidStr := nodes[2].IPFSAddStr(text)
		_ = nodes[3].IPFSAddStr(text)
		waitUntilProvidesComplete(t, nodes[3])

		cid, err := cid.Decode(cidStr)
		assert.NoError(t, err)

		c, err := client.New(nodes[1].GatewayURL())
		assert.NoError(t, err)

		resultsIter, err := c.FindProviders(context.Background(), cid)
		assert.NoError(t, err)

		records, err := iter.ReadAllResults(resultsIter)
		assert.NoError(t, err)

		var peers []peer.ID
		for _, record := range records {
			assert.Equal(t, types.SchemaPeer, record.GetSchema())

			peer, ok := record.(*types.PeerRecord)
			assert.True(t, ok)
			peers = append(peers, *peer.ID)
		}

		assert.Contains(t, peers, nodes[2].PeerID())
		assert.Contains(t, peers, nodes[3].PeerID())
	})

	t.Run("Get Peers Responds With Correct Peers", func(t *testing.T) {
		t.Parallel()
		nodes := setupNodes(t)

		c, err := client.New(nodes[1].GatewayURL())
		assert.NoError(t, err)

		resultsIter, err := c.FindPeers(context.Background(), nodes[2].PeerID())
		assert.NoError(t, err)

		records, err := iter.ReadAllResults(resultsIter)
		assert.NoError(t, err)
		assert.Len(t, records, 1)
		assert.IsType(t, records[0].GetSchema(), records[0].GetSchema())
		assert.IsType(t, records[0], &types.PeerRecord{})

		peer := records[0]
		assert.Equal(t, nodes[2].PeerID().String(), peer.ID.String())
		assert.NotEmpty(t, peer.Addrs)
	})

	t.Run("Get IPNS Record Responds With Correct Record", func(t *testing.T) {
		t.Parallel()
		nodes := setupNodes(t)

		text := "hello ipns test " + uuid.New().String()
		cidStr := nodes[0].IPFSAddStr(text)
		nodes[0].IPFS("name", "publish", "--allow-offline", cidStr)

		// Ask for record from a different peer.
		c, err := client.New(nodes[1].GatewayURL())
		assert.NoError(t, err)

		record, err := c.GetIPNS(context.Background(), ipns.NameFromPeer(nodes[0].PeerID()))
		assert.NoError(t, err)

		value, err := record.Value()
		assert.NoError(t, err)
		assert.Equal(t, "/ipfs/"+cidStr, value.String())
	})

	t.Run("Put IPNS Record Succeeds", func(t *testing.T) {
		t.Parallel()
		nodes := setupNodes(t)

		// Publish a record and confirm the /routing/v1/ipns API exposes the IPNS record
		text := "hello ipns test " + uuid.New().String()
		cidStr := nodes[0].IPFSAddStr(text)
		nodes[0].IPFS("name", "publish", "--allow-offline", cidStr)
		c, err := client.New(nodes[0].GatewayURL())
		assert.NoError(t, err)
		record, err := c.GetIPNS(context.Background(), ipns.NameFromPeer(nodes[0].PeerID()))
		assert.NoError(t, err)
		value, err := record.Value()
		assert.NoError(t, err)
		assert.Equal(t, "/ipfs/"+cidStr, value.String())

		// Start lonely node that is not connected to other nodes.
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Gateway.ExposeRoutingAPI = config.True
			cfg.Routing.Type = config.NewOptionalString("dht")
		})
		node.StartDaemon()
		defer node.StopDaemon()

		// Put IPNS record in lonely node. It should be accepted as it is a valid record.
		c, err = client.New(node.GatewayURL())
		assert.NoError(t, err)
		err = c.PutIPNS(context.Background(), ipns.NameFromPeer(nodes[0].PeerID()), record)
		assert.NoError(t, err)

		// Get the record from lonely node and double check.
		record, err = c.GetIPNS(context.Background(), ipns.NameFromPeer(nodes[0].PeerID()))
		assert.NoError(t, err)
		value, err = record.Value()
		assert.NoError(t, err)
		assert.Equal(t, "/ipfs/"+cidStr, value.String())
	})

	t.Run("GetClosestPeers returns error when DHT is disabled", func(t *testing.T) {
		t.Parallel()

		// Test various routing types that don't support DHT
		routingTypes := []string{"none", "delegated", "custom"}
		for _, routingType := range routingTypes {
			t.Run("routing_type="+routingType, func(t *testing.T) {
				t.Parallel()

				// Create node with specified routing type (DHT disabled)
				node := harness.NewT(t).NewNode().Init()
				node.UpdateConfig(func(cfg *config.Config) {
					cfg.Gateway.ExposeRoutingAPI = config.True
					cfg.Routing.Type = config.NewOptionalString(routingType)

					// For custom routing type, we need to provide minimal valid config
					// otherwise daemon startup will fail
					if routingType == "custom" {
						// Configure a minimal HTTP router (no DHT)
						cfg.Routing.Routers = map[string]config.RouterParser{
							"http-only": {
								Router: config.Router{
									Type: config.RouterTypeHTTP,
									Parameters: config.HTTPRouterParams{
										Endpoint: "https://delegated-ipfs.dev",
									},
								},
							},
						}
						cfg.Routing.Methods = map[config.MethodName]config.Method{
							config.MethodNameProvide:       {RouterName: "http-only"},
							config.MethodNameFindProviders: {RouterName: "http-only"},
							config.MethodNameFindPeers:     {RouterName: "http-only"},
							config.MethodNameGetIPNS:       {RouterName: "http-only"},
							config.MethodNamePutIPNS:       {RouterName: "http-only"},
						}
					}

					// For delegated routing type, ensure we have at least one HTTP router
					// to avoid daemon startup failure
					if routingType == "delegated" {
						// Use a minimal delegated router configuration
						cfg.Routing.DelegatedRouters = []string{"https://delegated-ipfs.dev"}
						// Delegated routing doesn't support providing, must be disabled
						cfg.Provide.Enabled = config.False
					}
				})
				node.StartDaemon()
				defer node.StopDaemon()

				c, err := client.New(node.GatewayURL())
				require.NoError(t, err)

				// Try to get closest peers - should fail gracefully with an error.
				// Use 60-second timeout (server has 30s routing timeout).
				testCid, err := cid.Decode("QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn")
				require.NoError(t, err)

				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()
				_, err = c.GetClosestPeers(ctx, testCid)
				require.Error(t, err)
				// All these routing types should indicate DHT is not available
				// The exact error message may vary based on implementation details
				errStr := err.Error()
				assert.True(t,
					strings.Contains(errStr, "not supported") ||
						strings.Contains(errStr, "not available") ||
						strings.Contains(errStr, "500"),
					"Expected error indicating DHT not available for routing type %s, got: %s", routingType, errStr)
			})
		}
	})

	t.Run("GetClosestPeers returns peers", func(t *testing.T) {
		t.Parallel()

		routingTypes := []string{"auto", "autoclient", "dht", "dhtclient"}

		// One node per routing type, all bootstrapping off the same set of
		// in-process DHT peers on loopback. Pointing this at the public swarm
		// instead made the test depend on a CI runner reaching
		// bootstrap.libp2p.io from a cold repo, which is what used to fail.
		h := harness.NewT(t)
		nodes := h.NewNodes(len(routingTypes)).Init()
		for i, routingType := range routingTypes {
			nodes[i].UpdateConfig(func(cfg *config.Config) {
				cfg.Gateway.ExposeRoutingAPI = config.True
				cfg.Routing.Type = config.NewOptionalString(routingType)
			})
		}
		h.BootstrapWithStubDHT(nodes)

		for i, routingType := range routingTypes {
			node := nodes[i]
			t.Run("routing_type="+routingType, func(t *testing.T) {
				t.Parallel()

				node.StartDaemon()
				defer node.StopDaemon()

				c, err := client.New(node.GatewayURL())
				require.NoError(t, err)

				// Query for closest peers to our own peer ID
				key := peer.ToCid(node.PeerID())

				// The stub peers are on loopback and always reachable, so the
				// WAN routing table fills in as soon as the daemon finishes
				// bootstrapping. The wait only covers that startup.
				var records []*types.PeerRecord
				require.EventuallyWithT(t, func(ct *assert.CollectT) {
					ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
					defer cancel()
					resultsIter, err := c.GetClosestPeers(ctx, key)
					if !assert.NoError(ct, err) {
						return
					}
					res, err := iter.ReadAllResults(resultsIter)
					if !assert.NoError(ct, err) {
						return
					}
					if !assert.NotEmpty(ct, res) {
						return
					}
					records = res
				}, 60*time.Second, 500*time.Millisecond)

				// Verify we got some peers back from WAN DHT
				require.NotEmpty(t, records, "should return peers close to own peerid")

				// Per IPIP-0476, GetClosestPeers returns at most 20 peers
				assert.LessOrEqual(t, len(records), 20, "IPIP-0476 limits GetClosestPeers to 20 peers")

				// Verify structure of returned records
				for _, record := range records {
					assert.Equal(t, types.SchemaPeer, record.Schema)
					assert.NotNil(t, record.ID)
					assert.NotEmpty(t, record.Addrs, "peer record should have addresses")
				}
			})
		}
	})
}
