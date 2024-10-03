package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/core/commands"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	libp2phttp "github.com/libp2p/go-libp2p/p2p/http"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/stretchr/testify/require"
)

func TestGatewayOverLibp2p(t *testing.T) {
	t.Parallel()
	nodes := harness.NewT(t).NewNodes(2).Init()

	// Setup streaming functionality
	nodes.ForEachPar(func(node *harness.Node) {
		node.IPFS("config", "--json", "Experimental.Libp2pStreamMounting", "true")
	})

	gwNode := nodes[0]
	p2pProxyNode := nodes[1]

	nodes.StartDaemons().Connect()

	// Add data to the gateway node
	cidDataOnGatewayNode := cid.MustParse(gwNode.IPFSAddStr("Hello Worlds2!"))
	r := gwNode.GatewayClient().Get(fmt.Sprintf("/ipfs/%s?format=raw", cidDataOnGatewayNode))
	blockDataOnGatewayNode := []byte(r.Body)

	// Add data to the non-gateway node
	cidDataNotOnGatewayNode := cid.MustParse(p2pProxyNode.IPFSAddStr("Hello Worlds!"))
	r = p2pProxyNode.GatewayClient().Get(fmt.Sprintf("/ipfs/%s?format=raw", cidDataNotOnGatewayNode))
	blockDataNotOnGatewayNode := []byte(r.Body)
	_ = blockDataNotOnGatewayNode

	// Setup one of the nodes as http to http-over-libp2p proxy
	p2pProxyNode.IPFS("p2p", "forward", "--allow-custom-protocol", "/http/1.1", "/ip4/127.0.0.1/tcp/0", fmt.Sprintf("/p2p/%s", gwNode.PeerID()))
	lsOutput := commands.P2PLsOutput{}
	if err := json.Unmarshal(p2pProxyNode.IPFS("p2p", "ls", "--enc=json").Stdout.Bytes(), &lsOutput); err != nil {
		t.Fatal(err)
	}
	require.Len(t, lsOutput.Listeners, 1)
	p2pProxyNodeHTTPListenMA, err := multiaddr.NewMultiaddr(lsOutput.Listeners[0].ListenAddress)
	require.NoError(t, err)

	p2pProxyNodeHTTPListenAddr, err := manet.ToNetAddr(p2pProxyNodeHTTPListenMA)
	require.NoError(t, err)

	t.Run("DoesNotWorkWithoutExperimentalConfig", func(t *testing.T) {
		_, err := http.Get(fmt.Sprintf("http://%s/ipfs/%s?format=raw", p2pProxyNodeHTTPListenAddr, cidDataOnGatewayNode))
		require.Error(t, err)
	})

	// Enable the experimental feature and reconnect the nodes
	gwNode.IPFS("config", "--json", "Experimental.GatewayOverLibp2p", "true")
	gwNode.StopDaemon().StartDaemon()
	nodes.Connect()

	// Note: the bare HTTP requests here assume that the gateway is mounted at `/`
	t.Run("WillNotServeRemoteContent", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("http://%s/ipfs/%s?format=raw", p2pProxyNodeHTTPListenAddr, cidDataNotOnGatewayNode))
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("WillNotServeDeserializedResponses", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("http://%s/ipfs/%s", p2pProxyNodeHTTPListenAddr, cidDataOnGatewayNode))
		require.NoError(t, err)
		require.Equal(t, http.StatusNotAcceptable, resp.StatusCode)
	})

	t.Run("ServeBlock", func(t *testing.T) {
		t.Run("UsingKuboProxy", func(t *testing.T) {
			resp, err := http.Get(fmt.Sprintf("http://%s/ipfs/%s?format=raw", p2pProxyNodeHTTPListenAddr, cidDataOnGatewayNode))
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, 200, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, blockDataOnGatewayNode, body)
		})
		t.Run("UsingLibp2pClientWithPathDiscovery", func(t *testing.T) {
			clientHost, err := libp2p.New(libp2p.NoListenAddrs)
			require.NoError(t, err)
			err = clientHost.Connect(context.Background(), peer.AddrInfo{
				ID:    gwNode.PeerID(),
				Addrs: gwNode.SwarmAddrs(),
			})
			require.NoError(t, err)

			client, err := (&libp2phttp.Host{StreamHost: clientHost}).NamespacedClient("/ipfs/gateway", peer.AddrInfo{ID: gwNode.PeerID()})
			require.NoError(t, err)

			resp, err := client.Get(fmt.Sprintf("/ipfs/%s?format=raw", cidDataOnGatewayNode))
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, 200, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			require.Equal(t, blockDataOnGatewayNode, body)
		})
	})
}
