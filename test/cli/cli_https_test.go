package cli

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/require"
)

func TestCLIWithRemoteHTTPS(t *testing.T) {
	tests := []struct{ addrSuffix string }{{"https"}, {"tls/http"}}
	for _, tt := range tests {
		t.Run("with "+tt.addrSuffix+" multiaddr", func(t *testing.T) {

			// Create HTTPS test server
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.TLS == nil {
					t.Error("Mocked Kubo RPC received plain HTTP request instead of HTTPS TLS Handshake")
				}
				_, _ = w.Write([]byte("OK"))
			}))
			defer server.Close()

			serverURL, _ := url.Parse(server.URL)
			_, port, _ := net.SplitHostPort(serverURL.Host)

			// Create Kubo repo
			node := harness.NewT(t).NewNode().Init()

			// Attempt to talk to remote Kubo RPC endpoint over HTTPS
			resp := node.RunIPFS("id", "--api", fmt.Sprintf("/ip4/127.0.0.1/tcp/%s/%s", port, tt.addrSuffix))

			// Expect HTTPS error (confirming TLS and https:// were used, and not Cleartext HTTP)
			require.Error(t, resp.Err)
			require.Contains(t, resp.Stderr.String(), "Error: tls: failed to verify certificate: x509: certificate signed by unknown authority")

			node.StopDaemon()

		})
	}
}
