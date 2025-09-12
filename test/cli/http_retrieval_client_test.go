package cli

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/ipfs/boxo/routing/http/server"
	"github.com/ipfs/boxo/routing/http/types"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-test/random"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/ipfs/kubo/test/cli/testutils/httprouting"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
)

func TestHTTPRetrievalClient(t *testing.T) {
	t.Parallel()

	// many moving pieces here, show more when debug is needed
	debug := os.Getenv("DEBUG") == "true"

	// usee local /routing/v1/providers/{cid} and
	// /ipfs/{cid} HTTP servers to confirm HTTP-only retrieval works end-to-end.
	t.Run("works end-to-end with an HTTP-only provider", func(t *testing.T) {
		// setup mocked HTTP Router to handle /routing/v1/providers/cid
		mockRouter := &httprouting.MockHTTPContentRouter{Debug: debug}
		delegatedRoutingServer := httptest.NewServer(server.Handler(mockRouter))
		t.Cleanup(func() { delegatedRoutingServer.Close() })

		// init Kubo repo
		node := harness.NewT(t).NewNode().Init()

		node.UpdateConfig(func(cfg *config.Config) {
			// explicitly enable http client
			cfg.HTTPRetrieval.Enabled = config.True
			// allow NewMockHTTPProviderServer to use self-signed TLS cert
			cfg.HTTPRetrieval.TLSInsecureSkipVerify = config.True
			// setup client-only routing which asks both HTTP + DHT
			// cfg.Routing.Type = config.NewOptionalString("autoclient")
			// setup Kubo node to use mocked HTTP Router
			cfg.Routing.DelegatedRouters = []string{delegatedRoutingServer.URL}
		})

		// compute a random CID
		randStr := string(random.Bytes(100))
		res := node.PipeStrToIPFS(randStr, "add", "-qn", "--cid-version", "1") // -n means dont add to local repo, just produce CID
		wantCIDStr := res.Stdout.Trimmed()
		testCid := cid.MustParse(wantCIDStr)

		// setup mock HTTP provider
		httpProviderServer := NewMockHTTPProviderServer(testCid, randStr, debug)
		t.Cleanup(func() { httpProviderServer.Close() })
		httpHost, httpPort, err := splitHostPort(httpProviderServer.URL)
		assert.NoError(t, err)

		// setup /routing/v1/providers/cid result that points at our mocked HTTP provider
		mockHTTPProviderPeerID := "12D3KooWCjfPiojcCUmv78Wd1NJzi4Mraj1moxigp7AfQVQvGLwH" // static, it does not matter, we only care about multiaddr
		mockHTTPMultiaddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%s/tls/http", httpHost, httpPort))
		mpid, _ := peer.Decode(mockHTTPProviderPeerID)
		mockRouter.AddProvider(testCid, &types.PeerRecord{
			Schema: types.SchemaPeer,
			ID:     &mpid,
			Addrs:  []types.Multiaddr{{Multiaddr: mockHTTPMultiaddr}},
			// no explicit Protocols, ensure multiaddr alone is enough
		})

		// Start Kubo
		node.StartDaemon()

		if debug {
			fmt.Printf("delegatedRoutingServer.URL: %s\n", delegatedRoutingServer.URL)
			fmt.Printf("httpProviderServer.URL: %s\n", httpProviderServer.URL)
			fmt.Printf("httpProviderServer.Multiaddr: %s\n", mockHTTPMultiaddr)
			fmt.Printf("testCid: %s\n", testCid)
		}

		// Now, make Kubo to read testCid. it was not added to local blockstore, so it has only one provider -- a HTTP server.

		// First, confirm delegatedRoutingServer returned HTTP provider
		findprovsRes := node.IPFS("routing", "findprovs", testCid.String())
		assert.Equal(t, mockHTTPProviderPeerID, findprovsRes.Stdout.Trimmed())

		// Ok, now attempt retrieval.
		// If there was no timeout and returned bytes match expected body, HTTP routing and retrieval worked end-to-end.
		catRes := node.IPFS("cat", testCid.String())
		assert.Equal(t, randStr, catRes.Stdout.Trimmed())
	})
}

// NewMockHTTPProviderServer pretends to be http provider that supports
// block response https://specs.ipfs.tech/http-gateways/trustless-gateway/#block-responses-application-vnd-ipld-raw
func NewMockHTTPProviderServer(c cid.Cid, body string, debug bool) *httptest.Server {
	expectedPathPrefix := "/ipfs/" + c.String()
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if debug {
			fmt.Printf("NewMockHTTPProviderServer GET %s\n", req.URL.Path)
		}
		if strings.HasPrefix(req.URL.Path, expectedPathPrefix) {
			w.Header().Set("Content-Type", "application/vnd.ipld.raw")
			w.WriteHeader(http.StatusOK)
			if req.Method == "GET" {
				_, err := w.Write([]byte(body))
				if err != nil {
					fmt.Fprintf(os.Stderr, "NewMockHTTPProviderServer GET %s error: %v\n", req.URL.Path, err)
				}
			}
		} else if strings.HasPrefix(req.URL.Path, "/ipfs/bafkqaaa") {
			// This is probe from https://specs.ipfs.tech/http-gateways/trustless-gateway/#dedicated-probe-paths
			w.Header().Set("Content-Type", "application/vnd.ipld.raw")
			w.WriteHeader(http.StatusOK)
		} else {
			http.Error(w, "Not Found", http.StatusNotFound)
		}
	})

	// Make it HTTP/2 with self-signed TLS cert
	srv := httptest.NewUnstartedServer(handler)
	srv.EnableHTTP2 = true
	srv.StartTLS()
	return srv
}

func splitHostPort(httpUrl string) (ipAddr string, port string, err error) {
	u, err := url.Parse(httpUrl)
	if err != nil {
		return "", "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", "", fmt.Errorf("invalid URL format: missing scheme or host")
	}
	ipAddr, port, err = net.SplitHostPort(u.Host)
	if err != nil {
		return "", "", fmt.Errorf("failed to split host and port from %q: %w", u.Host, err)
	}
	return ipAddr, port, nil
}
