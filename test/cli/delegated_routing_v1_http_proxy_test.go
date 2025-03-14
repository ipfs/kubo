package cli

import (
	"testing"

	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/ipfs/kubo/test/cli/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoutingV1Proxy(t *testing.T) {
	t.Parallel()

	setupNodes := func(t *testing.T) harness.Nodes {
		nodes := harness.NewT(t).NewNodes(3).Init()

		// Node 0 uses DHT and exposes the Routing API.  For the DHT
		// to actually work there will need to be another DHT-enabled
		// node.
		nodes[0].UpdateConfig(func(cfg *config.Config) {
			cfg.Gateway.ExposeRoutingAPI = config.True
			cfg.Discovery.MDNS.Enabled = false
			cfg.Routing.Type = config.NewOptionalString("dht")
		})
		nodes[0].StartDaemon()

		// Node 1 uses Node 0 as Routing V1 source, no DHT.
		nodes[1].UpdateConfig(func(cfg *config.Config) {
			cfg.Discovery.MDNS.Enabled = false
			cfg.Routing.Type = config.NewOptionalString("custom")
			cfg.Routing.Methods = config.Methods{
				config.MethodNameFindPeers:     {RouterName: "KuboA"},
				config.MethodNameFindProviders: {RouterName: "KuboA"},
				config.MethodNameGetIPNS:       {RouterName: "KuboA"},
				config.MethodNamePutIPNS:       {RouterName: "KuboA"},
				config.MethodNameProvide:       {RouterName: "KuboA"},
			}
			cfg.Routing.Routers = config.Routers{
				"KuboA": config.RouterParser{
					Router: config.Router{
						Type: config.RouterTypeHTTP,
						Parameters: &config.HTTPRouterParams{
							Endpoint: nodes[0].GatewayURL(),
						},
					},
				},
			}
		})
		nodes[1].StartDaemon()

		// This is the second DHT node. Only used so that the DHT is
		// operative.
		nodes[2].UpdateConfig(func(cfg *config.Config) {
			cfg.Gateway.ExposeRoutingAPI = config.True
			cfg.Discovery.MDNS.Enabled = false
			cfg.Routing.Type = config.NewOptionalString("dht")
		})
		nodes[2].StartDaemon()

		// Connect them.
		nodes.Connect()

		return nodes
	}

	t.Run("Kubo can find provider for CID via Routing V1", func(t *testing.T) {
		t.Parallel()
		nodes := setupNodes(t)

		cidStr := nodes[0].IPFSAddStr(testutils.RandomStr(1000))
		// Reprovide as initialProviderDelay still ongoing
		res := nodes[0].IPFS("routing", "reprovide")
		require.NoError(t, res.Err)
		res = nodes[1].IPFS("routing", "findprovs", cidStr)
		assert.Equal(t, nodes[0].PeerID().String(), res.Stdout.Trimmed())
	})

	t.Run("Kubo can find peer via Routing V1", func(t *testing.T) {
		t.Parallel()
		nodes := setupNodes(t)

		// Start lonely node that is not connected to other nodes.
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Discovery.MDNS.Enabled = false
			cfg.Routing.Type = config.NewOptionalString("dht")
		})
		node.StartDaemon()

		// Connect Node 0 to Lonely Node.
		nodes[0].Connect(node)

		// Node 1 must find Lonely Node through Node 0 Routing V1.
		res := nodes[1].IPFS("routing", "findpeer", node.PeerID().String())
		assert.Equal(t, node.SwarmAddrs()[0].String(), res.Stdout.Trimmed())
	})

	t.Run("Kubo can retrieve IPNS record via Routing V1", func(t *testing.T) {
		t.Parallel()
		nodes := setupNodes(t)

		nodeName := "/ipns/" + ipns.NameFromPeer(nodes[0].PeerID()).String()

		// Can't resolve the name as isn't published yet.
		res := nodes[1].RunIPFS("routing", "get", nodeName)
		require.Error(t, res.ExitErr)

		// Publish record on Node 0.
		path := "/ipfs/" + nodes[0].IPFSAddStr(testutils.RandomStr(1000))
		nodes[0].IPFS("name", "publish", "--allow-offline", path)

		// Get record on Node 1 (no DHT).
		res = nodes[1].IPFS("routing", "get", nodeName)
		record, err := ipns.UnmarshalRecord(res.Stdout.Bytes())
		require.NoError(t, err)
		value, err := record.Value()
		require.NoError(t, err)
		require.Equal(t, path, value.String())
	})

	t.Run("Kubo can resolve IPNS name via Routing V1", func(t *testing.T) {
		t.Parallel()
		nodes := setupNodes(t)

		nodeName := "/ipns/" + ipns.NameFromPeer(nodes[0].PeerID()).String()

		// Can't resolve the name as isn't published yet.
		res := nodes[1].RunIPFS("routing", "get", nodeName)
		require.Error(t, res.ExitErr)

		// Publish name.
		path := "/ipfs/" + nodes[0].IPFSAddStr(testutils.RandomStr(1000))
		nodes[0].IPFS("name", "publish", "--allow-offline", path)

		// Resolve IPNS name
		res = nodes[1].IPFS("name", "resolve", nodeName)
		require.Equal(t, path, res.Stdout.Trimmed())
	})

	t.Run("Kubo can provide IPNS record via Routing V1", func(t *testing.T) {
		t.Parallel()
		nodes := setupNodes(t)

		// Publish something on Node 1 (no DHT).
		nodeName := "/ipns/" + ipns.NameFromPeer(nodes[1].PeerID()).String()
		path := "/ipfs/" + nodes[1].IPFSAddStr(testutils.RandomStr(1000))
		nodes[1].IPFS("name", "publish", "--allow-offline", path)

		// Retrieve through Node 0.
		res := nodes[0].IPFS("routing", "get", nodeName)
		record, err := ipns.UnmarshalRecord(res.Stdout.Bytes())
		require.NoError(t, err)
		value, err := record.Value()
		require.NoError(t, err)
		require.Equal(t, path, value.String())
	})
}
