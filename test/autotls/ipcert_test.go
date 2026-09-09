package autotls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

// TestIPCertificateEndToEnd drives the broker-free half of AutoTLS: a node
// that is reachable on the port the certificate authority validates on asks
// for a certificate covering its own IP address and never registers a domain
// name with anyone.
//
//  1. certmagic orders a certificate for the node's own IP (RFC 8738)
//  2. the authority validates it with TLS-ALPN-01 against the node's own
//     WebSocket listener, which answers on the same port it serves peers on
//  3. the node announces /ip4/<ip>/tcp/<port>/tls/ws, with no p2p-forge
//     hostname anywhere in its address set
//  4. the same certificate serves the HTTPProvider gateway on /tls/http
//
// The only thing the test relaxes is the port: a real authority always uses
// 443, which a test cannot bind, so Pebble is pointed at the node's port
// instead. Everything else, including the reachability gate and the address
// rewriting, runs as it does in production.
func TestIPCertificateEndToEnd(t *testing.T) {
	// Not parallel: NewStack uses t.Setenv, and the two canaries share
	// process-wide env and CoreDNS state.

	// The port has to be agreed before anything starts: the node listens on
	// it, and Pebble's validation authority connects back to it.
	swarmPort := reserveLoopbackPort(t)
	stack := NewStack(t, WithVATLSPort(swarmPort))

	h := harness.NewT(t)
	h.IPFSBin = mustAbs(t, "../../cmd/ipfs/ipfs")
	node := h.NewNode().Init()
	node.UpdateConfig(func(cfg *config.Config) {
		cfg.Addresses.Swarm = []string{fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", swarmPort)}

		// HTTPProvider serves the trustless gateway behind the same
		// certificate, on the same port.
		cfg.HTTPProvider.Enabled = config.True
		cfg.HTTPProvider.AnnounceMultiaddrs = config.True

		cfg.AutoTLS.Enabled = config.True
		cfg.AutoTLS.AutoWSS = config.True
		cfg.AutoTLS.DomainSuffix = config.NewOptionalString(stack.ForgeDomain)
		cfg.AutoTLS.RegistrationEndpoint = config.NewOptionalString(stack.ForgeRegistrationEndpoint)
		cfg.AutoTLS.RegistrationToken = config.NewOptionalString(stack.ForgeAuthToken)
		cfg.AutoTLS.CAEndpoint = config.NewOptionalString(stack.ACMEEndpoint)
		cfg.AutoTLS.TrustedCARootsPEM = config.NewOptionalString(stack.PebbleCAPEM)
		cfg.AutoTLS.RegistrationDelay = config.NewOptionalDuration(0)
		// Loopback is not a public address and autonat has nothing to
		// confirm on a single-node network, so lift both checks.
		cfg.AutoTLS.AllowPrivateForgeAddrs = config.True

		cfg.AutoTLS.IPCerts = config.True
		cfg.AutoTLS.IPCertsPort = config.NewOptionalInteger(int64(swarmPort))
	})

	node.StartDaemon()
	defer node.StopDaemon()
	defer func() {
		if t.Failed() {
			t.Logf("daemon stderr (tail):\n%s", tailString(node.Daemon.Stderr.String(), 20000))
		}
	}()

	wantWS := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/tls/ws", swarmPort)
	announced := waitForAnnouncedAddr(t, node, 60*time.Second, wantWS)

	// The broker is not involved: nothing under the forge domain is
	// announced, and no brokered hostname was needed to get here.
	for _, a := range announced {
		require.NotContains(t, a.String(), stack.ForgeDomain,
			"announced a brokered address even though the node certified its own")
	}

	// HTTPProvider derives its endpoint from the WebSocket listener, so the
	// IP form has to carry over.
	wantHTTP := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/tls/http", swarmPort)
	require.Contains(t, addrStrings(announced), wantHTTP,
		"HTTPProvider endpoint missing for the address we certified")

	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM([]byte(stack.PebbleIssuanceRootPEM)),
		"failed to parse Pebble issuance root PEM")

	// A client dialing an IP literal sends no SNI, so this also covers the
	// path where the certificate is picked by the address rather than a name.
	conn, err := tls.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(swarmPort)), &tls.Config{
		RootCAs:    pool,
		ServerName: "127.0.0.1",
		NextProtos: []string{"h2", "http/1.1"},
	})
	require.NoError(t, err, "TLS handshake against the certified address")
	leaf := conn.ConnectionState().PeerCertificates[0]
	require.NoError(t, conn.Close())
	require.Empty(t, leaf.DNSNames, "certificate should cover an address, not a name")
	require.Len(t, leaf.IPAddresses, 1)
	require.Equal(t, "127.0.0.1", leaf.IPAddresses[0].String())

	// Fetch a real block over the same certificate to prove HTTPProvider is
	// live behind it, not just announced.
	addCID := cid.MustParse(node.IPFSAddStr("hello ip certs"))
	client := &http.Client{
		Transport: &http2.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}},
		Timeout:   15 * time.Second,
	}
	url := fmt.Sprintf("https://127.0.0.1:%d/ipfs/%s?format=raw", swarmPort, addCID)
	resp, err := client.Get(url)
	require.NoError(t, err, "fetch %s", url)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "HTTP/2.0", resp.Proto)
	require.Equal(t, "application/vnd.ipld.raw", resp.Header.Get("Content-Type"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "hello ip certs")
}

// reserveLoopbackPort picks a free TCP port on loopback. It carries the usual
// race of handing a closed port to somebody else, which the existing forge
// registration endpoint accepts for the same reason: three parties have to
// agree on the port before any of them starts.
func reserveLoopbackPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	return port
}

// waitForAnnouncedAddr polls `ipfs id` until want shows up in the announced
// set, and returns everything announced at that point. Issuance takes a moment
// and the address only appears once the certificate is installed.
func waitForAnnouncedAddr(t *testing.T, n *harness.Node, timeout time.Duration, want string) []multiaddr.Multiaddr {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		addrs := readAnnouncedAddrs(t, n)
		for _, a := range addrs {
			if a.String() == want {
				return addrs
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("%s did not appear within %s; announced=%v", want, timeout, readAnnouncedAddrs(t, n))
	return nil
}

func addrStrings(addrs []multiaddr.Multiaddr) []string {
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, a.String())
	}
	return out
}
