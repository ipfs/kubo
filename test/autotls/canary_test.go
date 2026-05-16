package autotls

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

// TestACMEEndToEnd drives kubo's full AutoTLS chain in-process:
//
//  1. p2p-forge registration (PeerID-auth POST)
//  2. certmagic ACME order against an in-process Pebble
//  3. DNS-01 challenge validated through the p2p-forge DNS plugin
//  4. cert installation into the WebSocket transport
//  5. HTTP/2 fetch of a real block via the announced /tls/http multiaddr
//
// Every link above is load-bearing for any AutoTLS-enabled kubo node, so one
// canary covers all of them. Heavy deps (Pebble, CoreDNS) live in this
// sub-module's go.mod and stay out of kubo's main module; CI runs the canary
// as a dedicated autotls-tests job. See doc.go.
func TestACMEEndToEnd(t *testing.T) {
	t.Parallel()

	stack := NewStack(t)

	// The kubo harness finds the binary by walking up to the nearest
	// go.mod, but this sub-module has its own go.mod, so the walk stops
	// here. Point IPFSBin at the real kubo build at the repo root
	// (../../cmd/ipfs/ipfs from this sub-module's perspective).
	h := harness.NewT(t)
	h.IPFSBin = mustAbs(t, "../../cmd/ipfs/ipfs")
	node := h.NewNode().Init()
	node.UpdateConfig(func(cfg *config.Config) {
		// HTTPProvider exposes the trustless gateway over /tls/ws and
		// /tls/http using the AutoTLS cert.
		cfg.HTTPProvider.Enabled = config.True

		// Point AutoTLS at the in-process Pebble + p2p-forge.
		cfg.AutoTLS.Enabled = config.True
		cfg.AutoTLS.DomainSuffix = config.NewOptionalString(stack.ForgeDomain)
		cfg.AutoTLS.RegistrationEndpoint = config.NewOptionalString(stack.ForgeRegistrationEndpoint)
		cfg.AutoTLS.RegistrationToken = config.NewOptionalString(stack.ForgeAuthToken)
		cfg.AutoTLS.CAEndpoint = config.NewOptionalString(stack.ACMEEndpoint)
		cfg.AutoTLS.TrustedCARootsPEM = config.NewOptionalString(stack.PebbleCAPEM)
		cfg.AutoTLS.AllowPrivateForgeAddrs = config.True
		// Skip the production registration-delay so the test doesn't
		// have to wait for the libp2p reachability event.
		cfg.AutoTLS.RegistrationDelay = config.NewOptionalDuration(0)

		// AutoWSS adds /tls/sni/*.libp2p.test/ws to each /tcp/N, which
		// is what HTTPProvider's /tls/http announcement is derived from.
		cfg.AutoTLS.AutoWSS = config.True
	})

	node.StartDaemon()
	defer node.StopDaemon()

	// Wait for AutoTLS to obtain a cert and the resulting /tls/http
	// multiaddr to show up in the announced address set.
	defer func() {
		if t.Failed() {
			t.Logf("daemon stderr (tail):\n%s", tailString(node.Daemon.Stderr.String(), 20000))
		}
	}()
	announced := waitForTLSHTTPMultiaddr(t, node, 60*time.Second, stack.ForgeDomain)

	// Translate the /tls/http multiaddr into an HTTPS URL and a dial
	// address (raw IP:port we actually connect to; SNI carries the
	// forge-issued hostname).
	hostname, port := splitTLSHTTPAddr(t, announced)
	addInfo := node.IPFSAddStr("hello world")
	addCID := cid.MustParse(addInfo)
	url := fmt.Sprintf("https://%s:%s/ipfs/%s?format=raw", hostname, port, addCID)

	// Build an HTTP/2 client that trusts Pebble's ACME issuance root.
	// Cert chain validation goes all the way: Pebble-signed end-entity
	// cert, chained from Pebble's issuance root, served from a hostname
	// under stack.ForgeDomain.
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM([]byte(stack.PebbleIssuanceRootPEM)),
		"failed to parse Pebble issuance root PEM")
	client := &http.Client{
		Transport: &http2.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    pool,
				ServerName: hostname,
			},
			DialTLS: func(network, _ string, cfg *tls.Config) (net.Conn, error) {
				raw, err := net.Dial(network, net.JoinHostPort("127.0.0.1", port))
				if err != nil {
					return nil, err
				}
				c := tls.Client(raw, cfg)
				if err := c.Handshake(); err != nil {
					_ = raw.Close()
					return nil, err
				}
				return c, nil
			},
		},
		Timeout: 15 * time.Second,
	}

	resp, err := client.Get(url)
	require.NoError(t, err, "fetch /ipfs/%s via %s", addCID, url)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "HTTP/2.0", resp.Proto)
	require.Equal(t, "application/vnd.ipld.raw", resp.Header.Get("Content-Type"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	// `ipfs add` of a small string produces a UnixFS file, so the raw
	// block is a UnixFS protobuf with the literal payload embedded. We
	// don't unmarshal it here; the canary's job is to prove the AutoTLS
	// path round-trips real block bytes, not to re-test UnixFS.
	require.Contains(t, string(body), "hello world",
		"raw block payload should embed the added bytes")
}

// waitForTLSHTTPMultiaddr polls `ipfs id` until a multiaddr that ends in
// /tls/http and references the forge domain appears, or the deadline
// expires. Pebble's cert issuance takes a second or two on the first run;
// the AddrsFactory derivation of /tls/http happens once the AutoTLS cert
// is installed and the libp2p host's listen addresses are re-evaluated.
func waitForTLSHTTPMultiaddr(t *testing.T, n *harness.Node, timeout time.Duration, forgeDomain string) multiaddr.Multiaddr {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, a := range readAnnouncedAddrs(t, n) {
			s := a.String()
			if !strings.HasSuffix(s, "/tls/http") {
				continue
			}
			if !strings.Contains(s, forgeDomain) {
				continue
			}
			return a
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("no /tls/http multiaddr appeared within %s; announced=%v",
		timeout, readAnnouncedAddrs(t, n))
	return nil
}

// readAnnouncedAddrs returns the multiaddrs the node advertises via
// identify (output of `ipfs id`). This is what other peers see, and
// includes the /tls/http addresses derived by HTTPProvider.
func readAnnouncedAddrs(t *testing.T, n *harness.Node) []multiaddr.Multiaddr {
	t.Helper()
	out := n.IPFS("id", "--enc=json").Stdout.String()
	var idDoc struct {
		Addresses []string `json:"Addresses"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &idDoc))
	addrs := make([]multiaddr.Multiaddr, 0, len(idDoc.Addresses))
	for _, s := range idDoc.Addresses {
		if i := strings.Index(s, "/p2p/"); i >= 0 {
			s = s[:i]
		}
		a, err := multiaddr.NewMultiaddr(s)
		if err != nil {
			t.Fatalf("parse announced addr %q: %v", s, err)
		}
		addrs = append(addrs, a)
	}
	return addrs
}

// splitTLSHTTPAddr extracts (hostname, port) from a /dns*/<host>/tcp/<port>/
// tls/sni/<host>/http or similar multiaddr. The hostname is what SNI must
// be set to; the port is what we actually dial on 127.0.0.1.
func splitTLSHTTPAddr(t *testing.T, m multiaddr.Multiaddr) (host, port string) {
	t.Helper()
	// SNI component (if present) wins for the hostname, otherwise the
	// /dns* value. Both forms appear depending on AutoTLS.ShortAddrs.
	for _, code := range []int{multiaddr.P_SNI, multiaddr.P_DNS, multiaddr.P_DNS4, multiaddr.P_DNS6, multiaddr.P_DNSADDR} {
		v, err := m.ValueForProtocol(code)
		if err == nil && v != "" {
			host = v
			break
		}
	}
	require.NotEmpty(t, host, "no hostname component in %s", m)
	p, err := m.ValueForProtocol(multiaddr.P_TCP)
	require.NoError(t, err, "no tcp port in %s", m)
	return host, p
}

// mustAbs returns the absolute path of rel, panicking via require if the
// filesystem call fails. Used to compute the kubo binary location from
// this sub-module's cwd.
func mustAbs(t *testing.T, rel string) string {
	t.Helper()
	abs, err := filepath.Abs(rel)
	require.NoError(t, err)
	return abs
}

// tailString returns the last n bytes of s, prefixed with "...".
func tailString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "... (truncated)\n" + s[len(s)-n:]
}

