package cli

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	gws "github.com/gorilla/websocket"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

// TestHTTPProviderAutoTLS exercises the AutoTLS HTTPS path of HTTPProvider
// using a self-signed test cert (AutoTLS.SelfSignedForTests=true). Test
// clients pair this with InsecureSkipVerify to skip cert chain validation.
//
// The full p2p-forge + ACME chain is covered separately by the canary in
// http_provider_autotls_e2e_test.go; the cases here focus on listener
// wiring, the kubo-side h2-over-TLS policy, and the multiaddr/well-known
// surfaces that browsers and HTTP retrieval clients depend on.
func TestHTTPProviderAutoTLS(t *testing.T) {
	t.Parallel()
	nodes := harness.NewT(t).NewNodes(2).Init()
	gwNode := nodes[0]
	otherNode := nodes[1]

	// Add a /tls/ws listener. AutoWSS normally adds /tls/sni/<host>/ws
	// for AutoTLS; the test bypasses AutoTLS in favour of the
	// SelfSignedForTests path, so a plain /tls/ws is enough. The client
	// uses InsecureSkipVerify, so SNI is decorative.
	gwNode.UpdateConfig(func(cfg *config.Config) {
		cfg.Addresses.Swarm = append(cfg.Addresses.Swarm,
			"/ip4/127.0.0.1/tcp/0/tls/ws")
		cfg.AutoTLS.Enabled = config.False
		cfg.AutoTLS.SelfSignedForTests = config.True
		cfg.HTTPProvider.Enabled = config.True
		// AnnouncesTLSHTTPMultiaddr below needs the /tls/http announcement.
		cfg.HTTPProvider.AnnounceMultiaddrs = config.True
	})

	nodes.StartDaemons().Connect()
	defer nodes.StopDaemons()
	t.Cleanup(func() { gwNode.StopDaemon() })

	cidLocal := cid.MustParse(gwNode.IPFSAddStr("Hello AutoTLS HTTPProvider!"))
	expectedRawBlock := []byte(gwNode.GatewayClient().Get(
		fmt.Sprintf("/ipfs/%s?format=raw", cidLocal),
	).Body)
	cidRemote := cid.MustParse(otherNode.IPFSAddStr("not on the gateway"))

	hostPort := findTLSWSListenHostPort(t, gwNode)
	// SNI must match the cert's wildcard SAN; the listener was configured
	// to match the example.libp2p.direct subdomain.
	const tlsServerName = "example.libp2p.direct"
	baseURL := "https://" + tlsServerName + ":" + portOf(t, hostPort)
	dialAddr := hostPort // raw IP:port we actually dial

	t.Run("ServeBlock_HTTPSh2", func(t *testing.T) {
		// h2 over TLS: the gateway handler returns the raw block.
		resp := mustGetHTTPS(t, baseURL+"/ipfs/"+cidLocal.String()+"?format=raw",
			[]string{"h2", "http/1.1"}, dialAddr, tlsServerName)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "HTTP/2.0", resp.Proto)
		require.Equal(t, "application/vnd.ipld.raw", resp.Header.Get("Content-Type"))
		require.Equal(t, expectedRawBlock, mustReadBody(t, resp))
	})

	t.Run("HTTP1OverTLSRejected", func(t *testing.T) {
		// kubo's RequireHTTP2OverTLS policy: HTTP/1.1 over TLS is
		// reserved for the WebSocket upgrade. Plain h1 GET gets 426
		// with an Upgrade header pointing the client at h2 or
		// websocket.
		resp := mustGetHTTPS(t, baseURL+"/ipfs/"+cidLocal.String()+"?format=raw",
			[]string{"http/1.1"}, dialAddr, tlsServerName)
		require.Equal(t, http.StatusUpgradeRequired, resp.StatusCode)
		require.Contains(t, resp.Header.Values("Upgrade"), "h2,websocket")
	})

	t.Run("WSSUpgradeStillWorks", func(t *testing.T) {
		// Browser-equivalent path: ALPN http/1.1 only, classic
		// WebSocket Upgrade. Must complete with 101 Switching
		// Protocols even when the listener also serves HTTP/2 GETs.
		dialer := gws.Dialer{
			NetDial: func(network, _ string) (net.Conn, error) {
				return net.Dial(network, dialAddr)
			},
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				ServerName:         tlsServerName,
				NextProtos:         []string{"http/1.1"},
			},
			HandshakeTimeout: 5 * time.Second,
		}
		conn, resp, err := dialer.Dial("wss://"+tlsServerName+":"+portOf(t, hostPort)+"/", nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
		require.NoError(t, conn.Close())
	})

	t.Run("WillNotServeRemoteContent", func(t *testing.T) {
		resp := mustGetHTTPS(t, baseURL+"/ipfs/"+cidRemote.String()+"?format=raw",
			[]string{"h2"}, dialAddr, tlsServerName)
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("ServesWellKnownProtocolsOverHTTPS", func(t *testing.T) {
		resp := mustGetHTTPS(t, baseURL+"/.well-known/libp2p/protocols",
			[]string{"h2"}, dialAddr, tlsServerName)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
		var doc map[string]map[string]string
		require.NoError(t, json.Unmarshal(mustReadBody(t, resp), &doc))
		require.Contains(t, doc, "/ipfs/gateway")
	})

	t.Run("AnnouncesTLSHTTPMultiaddr", func(t *testing.T) {
		announced := announcedAddrs(t, gwNode)
		var tlsWS, tlsHTTP []multiaddr.Multiaddr
		for _, a := range announced {
			s := a.String()
			switch {
			case strings.HasSuffix(s, "/tls/ws") ||
				strings.Contains(s, "/tls/sni/") && strings.HasSuffix(s, "/ws"):
				tlsWS = append(tlsWS, a)
			case strings.HasSuffix(s, "/tls/http") ||
				strings.Contains(s, "/tls/sni/") && strings.HasSuffix(s, "/http"):
				tlsHTTP = append(tlsHTTP, a)
			}
		}
		require.NotEmpty(t, tlsWS, "expected at least one /tls/ws in announced addrs")
		require.Equal(t, len(tlsWS), len(tlsHTTP),
			"every announced /tls/ws should have a sibling /tls/http (ws=%v http=%v)",
			addrStrings(tlsWS), addrStrings(tlsHTTP))
	})
}

// findTLSWSListenHostPort returns "host:port" for a /tls/ws listener (in any
// of its forms: /tls/ws, /tls/sni/<host>/ws). The TCP port is what we dial;
// the SNI is set separately on the TLS client config.
func findTLSWSListenHostPort(t *testing.T, n *harness.Node) string {
	t.Helper()
	for _, a := range n.SwarmAddrs() {
		s := a.String()
		if !strings.HasSuffix(s, "/ws") || !strings.Contains(s, "/tls/") {
			continue
		}
		host, err := a.ValueForProtocol(multiaddr.P_IP4)
		require.NoError(t, err)
		port, err := a.ValueForProtocol(multiaddr.P_TCP)
		require.NoError(t, err)
		return net.JoinHostPort(host, port)
	}
	t.Fatalf("no /tls/ws listener found on node %d; addrs=%v", n.ID, addrStrings(n.SwarmAddrs()))
	return ""
}

// portOf extracts the port from a host:port string.
func portOf(t *testing.T, hostPort string) string {
	t.Helper()
	_, port, err := net.SplitHostPort(hostPort)
	require.NoError(t, err)
	return port
}

// mustGetHTTPS issues a TLS GET, dialing dialAddr (raw IP:port) but
// presenting tlsServerName as SNI and Host. ALPN is restricted to
// nextProtos so tests can pin which HTTP version they want negotiated.
// InsecureSkipVerify is on because the daemon serves a self-signed cert.
func mustGetHTTPS(t *testing.T, url string, nextProtos []string, dialAddr, tlsServerName string) *http.Response {
	t.Helper()
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         tlsServerName,
		NextProtos:         nextProtos,
	}
	dialContext := func(network, _ string) (net.Conn, error) {
		return net.Dial(network, dialAddr)
	}
	var rt http.RoundTripper
	switch {
	case len(nextProtos) > 0 && nextProtos[0] == "h2":
		// HTTP/2 only.
		rt = &http2.Transport{
			TLSClientConfig: tlsConf,
			DialTLS: func(network, _ string, cfg *tls.Config) (net.Conn, error) {
				raw, err := dialContext(network, "")
				if err != nil {
					return nil, err
				}
				tlsConn := tls.Client(raw, cfg)
				if err := tlsConn.Handshake(); err != nil {
					_ = raw.Close()
					return nil, err
				}
				return tlsConn, nil
			},
		}
	default:
		// HTTP/1.1 (or h2/h1 with ALPN selection).
		rt = &http.Transport{
			ForceAttemptHTTP2: false,
			TLSClientConfig:   tlsConf,
			Dial:              dialContext,
		}
	}
	c := &http.Client{Transport: rt, Timeout: 10 * time.Second}
	resp, err := c.Get(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}
