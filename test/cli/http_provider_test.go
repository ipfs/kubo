package cli

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	gws "github.com/gorilla/websocket"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	libp2phttp "github.com/libp2p/go-libp2p/p2p/http"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

// TestHTTPProvider exercises the cleartext HTTP transport of the HTTPProvider
// feature: the trustless gateway handler reachable over plain /ws and /http
// on the same TCP port shared with libp2p WebSocket traffic. The TLS path is
// covered separately in http_provider_autotls_test.go.
func TestHTTPProvider(t *testing.T) {
	t.Parallel()
	nodes := harness.NewT(t).NewNodes(2).Init()
	gwNode := nodes[0]
	otherNode := nodes[1]

	// Add a cleartext /ws listener on the gateway node so the HTTPProvider
	// handler has somewhere to live without needing AutoTLS in this test.
	// Sharing port 0 with the existing /tcp/0 means tcpreuse routes raw
	// libp2p, plain HTTP, and h2c into the same socket — the production
	// layout, just without TLS.
	gwNode.UpdateConfig(func(cfg *config.Config) {
		cfg.Addresses.Swarm = append(cfg.Addresses.Swarm, "/ip4/127.0.0.1/tcp/0/ws")
	})

	nodes.StartDaemons().Connect()
	defer nodes.StopDaemons()

	// Add data on the gateway node and capture the local raw block so
	// later byte-by-byte comparisons are exact.
	cidLocal := cid.MustParse(gwNode.IPFSAddStr("Hello HTTPProvider!"))
	expectedRawBlock := []byte(gwNode.GatewayClient().Get(
		fmt.Sprintf("/ipfs/%s?format=raw", cidLocal),
	).Body)

	// And on the other node, so we have a CID the gateway does NOT have
	// locally for the NoFetch test.
	cidRemote := cid.MustParse(otherNode.IPFSAddStr("not on the gateway"))

	wsHostPort := findWSListenHostPort(t, gwNode)
	baseURL := "http://" + wsHostPort

	t.Run("DoesNotWorkWithoutHTTPProvider", func(t *testing.T) {
		// HTTPProvider is off by default; the listener answers 404 for
		// non-WebSocket requests because no fallback handler is wired.
		resp := mustGetH1(t, baseURL+"/ipfs/"+cidLocal.String()+"?format=raw")
		require.Equal(t, http.StatusNotFound, resp.StatusCode,
			"non-upgrade HTTP requests must 404 until HTTPProvider is enabled")
	})

	// AnnouncesHTTPMultiaddr below needs both Enabled and
	// AnnounceMultiaddrs. Restart so FX picks up the handler.
	gwNode.IPFS("config", "--json", "HTTPProvider.Enabled", "true")
	gwNode.IPFS("config", "--json", "HTTPProvider.AnnounceMultiaddrs", "true")
	gwNode.StopDaemon().StartDaemon()
	t.Cleanup(func() { gwNode.StopDaemon() })
	nodes.Connect()

	// Refresh: the random port may differ across daemon restarts.
	wsHostPort = findWSListenHostPort(t, gwNode)
	baseURL = "http://" + wsHostPort

	t.Run("WillNotServeRemoteContent", func(t *testing.T) {
		// NoFetch invariant: the gateway node does not have cidRemote
		// locally and must not reach out to fetch it.
		resp := mustGetH1(t, baseURL+"/ipfs/"+cidRemote.String()+"?format=raw")
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("WillNotServeDeserializedResponses", func(t *testing.T) {
		// DeserializedResponses=false invariant: a request without
		// ?format=raw|car asks for deserialized UnixFS, which the
		// trustless gateway refuses.
		resp := mustGetH1(t, baseURL+"/ipfs/"+cidLocal.String())
		require.Equal(t, http.StatusNotAcceptable, resp.StatusCode)
	})

	t.Run("ServeBlock_HTTP1", func(t *testing.T) {
		resp := mustGetH1(t, baseURL+"/ipfs/"+cidLocal.String()+"?format=raw")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "HTTP/1.1", resp.Proto)
		require.Equal(t, "application/vnd.ipld.raw", resp.Header.Get("Content-Type"))
		require.Equal(t, expectedRawBlock, mustReadBody(t, resp))
	})

	t.Run("ServeBlock_H2C", func(t *testing.T) {
		// Reverse-proxy interop: h2c gives multiplexing without TLS.
		resp := mustGetH2C(t, baseURL+"/ipfs/"+cidLocal.String()+"?format=raw")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "HTTP/2.0", resp.Proto)
		require.Equal(t, "application/vnd.ipld.raw", resp.Header.Get("Content-Type"))
		require.Equal(t, expectedRawBlock, mustReadBody(t, resp))
	})

	t.Run("ServesWellKnownProtocols", func(t *testing.T) {
		// libp2p Gateway spec advertisement, reachable from any HTTP
		// client without opening a libp2p stream.
		resp := mustGetH1(t, baseURL+"/.well-known/libp2p/protocols")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
		var doc map[string]map[string]string
		require.NoError(t, json.Unmarshal(mustReadBody(t, resp), &doc))
		require.Contains(t, doc, "/ipfs/gateway")
		require.Equal(t, "/", doc["/ipfs/gateway"]["path"])
	})

	t.Run("WSUpgradeStillWorks", func(t *testing.T) {
		// The fallback handler must not break the original libp2p WS
		// upgrade. A 101 Switching Protocols is enough; we don't
		// complete the libp2p multistream handshake here.
		dialer := gws.Dialer{HandshakeTimeout: 5 * time.Second}
		conn, resp, err := dialer.Dial("ws://"+wsHostPort+"/", nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
		require.NoError(t, conn.Close())
	})

	t.Run("AnnouncesHTTPMultiaddr", func(t *testing.T) {
		// /http must appear next to every announced /ws. The /http
		// multiaddr is purely an announcement and has no socket of
		// its own, so read it via `ipfs id` rather than SwarmAddrs.
		announced := announcedAddrs(t, gwNode)
		var wsAddrs, httpAddrs []multiaddr.Multiaddr
		for _, a := range announced {
			s := a.String()
			switch {
			case strings.HasSuffix(s, "/ws"):
				wsAddrs = append(wsAddrs, a)
			case strings.HasSuffix(s, "/http"):
				httpAddrs = append(httpAddrs, a)
			}
		}
		require.NotEmpty(t, wsAddrs, "expected at least one /ws in announced addrs")
		require.Equal(t, len(wsAddrs), len(httpAddrs),
			"every announced /ws should have a sibling /http (ws=%v http=%v)",
			addrStrings(wsAddrs), addrStrings(httpAddrs))
	})

	t.Run("ServesWellKnownOverLibp2pStream", func(t *testing.T) {
		// Sanity check that the libp2p-stream gateway also lands the
		// gateway handler over the discovered protocol path. Mirrors
		// http_provider_over_libp2p_test.go's libp2p-client subtest
		// but uses libp2phttp's NamespacedClient discovery.
		clientHost, err := libp2p.New(libp2p.NoListenAddrs)
		require.NoError(t, err)
		t.Cleanup(func() { _ = clientHost.Close() })
		require.NoError(t, clientHost.Connect(context.Background(), peer.AddrInfo{
			ID:    gwNode.PeerID(),
			Addrs: gwNode.SwarmAddrs(),
		}))
		client, err := (&libp2phttp.Host{StreamHost: clientHost}).
			NamespacedClient("/ipfs/gateway", peer.AddrInfo{ID: gwNode.PeerID()})
		require.NoError(t, err)
		resp, err := client.Get(fmt.Sprintf("/ipfs/%s?format=raw", cidLocal))
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, expectedRawBlock, body)
	})
}

// findWSListenHostPort returns "host:port" for any /tcp/N/ws listener bound
// by the node. Auto-appended /ws shares the TCP port with raw /tcp/N via
// tcpreuse, so any matching listener is a fine target.
func findWSListenHostPort(t *testing.T, n *harness.Node) string {
	t.Helper()
	for _, a := range n.SwarmAddrs() {
		s := a.String()
		// Match exactly /tcp/<digits>/ws (cleartext); skip /tls/ws etc.
		if !strings.HasSuffix(s, "/ws") || strings.Contains(s, "/tls/") || strings.Contains(s, "/wss") {
			continue
		}
		host, err := a.ValueForProtocol(multiaddr.P_IP4)
		require.NoError(t, err)
		port, err := a.ValueForProtocol(multiaddr.P_TCP)
		require.NoError(t, err)
		return net.JoinHostPort(host, port)
	}
	t.Fatalf("no cleartext /ws listener found on node %d; addrs=%v", n.ID, addrStrings(n.SwarmAddrs()))
	return ""
}

// announcedAddrs returns the multiaddrs this node advertises via identify
// (output of `ipfs id`), which is what other peers see in the DHT and via
// peer-routing. Differs from SwarmAddrs() in that it includes addresses
// that are advertised but not bound (the HTTPProvider /http multiaddr is
// the canonical example).
func announcedAddrs(t *testing.T, n *harness.Node) []multiaddr.Multiaddr {
	t.Helper()
	out := n.IPFS("id", "--enc=json").Stdout.String()
	var idDoc struct {
		Addresses []string `json:"Addresses"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &idDoc))
	addrs := make([]multiaddr.Multiaddr, 0, len(idDoc.Addresses))
	for _, s := range idDoc.Addresses {
		// Strip the trailing /p2p/<peerid> wrapper for easier suffix
		// matching downstream.
		if i := strings.Index(s, "/p2p/"); i >= 0 {
			s = s[:i]
		}
		a, err := multiaddr.NewMultiaddr(s)
		if err != nil {
			t.Fatalf("parse announced addr %q: %s", s, err)
		}
		addrs = append(addrs, a)
	}
	return addrs
}

// addrStrings is a small log-friendly view of a multiaddr slice.
func addrStrings(addrs []multiaddr.Multiaddr) []string {
	out := make([]string, len(addrs))
	for i, a := range addrs {
		out[i] = a.String()
	}
	return out
}

// mustGetH1 performs a plain HTTP/1.1 GET. Caller must close resp.Body.
func mustGetH1(t *testing.T, url string) *http.Response {
	t.Helper()
	tr := &http.Transport{
		// Force HTTP/1.1: an empty NextProtos in TLS skips ALPN, but
		// for cleartext h1 is the default.
	}
	defer tr.CloseIdleConnections()
	c := &http.Client{Transport: tr, Timeout: 10 * time.Second}
	resp, err := c.Get(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// mustGetH2C performs a prior-knowledge HTTP/2 cleartext (h2c) GET. The
// AllowHTTP+DialTLSContext combo is the canonical way to drive h2c from a
// Go client — it tells http2.Transport to skip ALPN negotiation and assume
// the server speaks h2c on the wire.
func mustGetH2C(t *testing.T, url string) *http.Response {
	t.Helper()
	tr := &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(_ context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		},
	}
	defer tr.CloseIdleConnections()
	c := &http.Client{Transport: tr, Timeout: 10 * time.Second}
	resp, err := c.Get(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// mustReadBody drains and returns the response body.
func mustReadBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return body
}
