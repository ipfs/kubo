package libp2p

import (
	"crypto/x509"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSelfSignedTestTLSConfig spot-checks that the test-only cert covers
// the SANs the test harness depends on (loopback IPs and the AutoTLS
// wildcard) and is valid for at least the minute it takes a test to run.
func TestSelfSignedTestTLSConfig(t *testing.T) {
	cfg, err := NewSelfSignedTestTLSConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Len(t, cfg.Certificates, 1)

	leaf, err := x509.ParseCertificate(cfg.Certificates[0].Certificate[0])
	require.NoError(t, err)

	require.True(t, leaf.NotAfter.After(time.Now().Add(time.Minute)),
		"cert should be valid past the test window")
	require.True(t, leaf.NotBefore.Before(time.Now()),
		"cert should already be valid")

	require.True(t, ipAddressesContain(leaf.IPAddresses, "127.0.0.1"),
		"cert must cover loopback IPv4")
	require.True(t, ipAddressesContain(leaf.IPAddresses, "::1"),
		"cert must cover loopback IPv6")
	require.True(t, slices.Contains(leaf.DNSNames, "localhost"))
	require.True(t, slices.Contains(leaf.DNSNames, "*.libp2p.direct"),
		"cert must cover the AutoTLS wildcard so /tls/sni/<host>/ws listeners work in tests")
}

func ipAddressesContain(haystack []net.IP, needle string) bool {
	target := net.ParseIP(needle)
	for _, ip := range haystack {
		if ip.Equal(target) {
			return true
		}
	}
	return false
}
