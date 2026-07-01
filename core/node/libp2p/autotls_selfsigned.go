package libp2p

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"time"
)

// SelfSignedTestTLSConfig is a *tls.Config carrying a freshly generated
// self-signed cert. Wrapped in a named type so FX can disambiguate it from
// any other *tls.Config value in the dependency graph.
type SelfSignedTestTLSConfig struct {
	*tls.Config
}

// NewSelfSignedTestTLSConfig generates an in-memory self-signed certificate
// and returns it wrapped as the FX-friendly SelfSignedTestTLSConfig.
//
// The cert covers `localhost`, `127.0.0.1`, and `::1`, plus the AutoTLS
// wildcard `*.libp2p.direct` so /tls/sni/<host>/ws listeners that the test
// configures by hand still match. The key is freshly generated each daemon
// start. Test clients must use tls.Config{InsecureSkipVerify: true} since
// no public CA signs this certificate.
//
// This provider is wired only when AutoTLS.SelfSignedForTests is true.
// It is a test escape hatch and must not be used in production.
func NewSelfSignedTestTLSConfig() (*SelfSignedTestTLSConfig, error) {
	cert, err := generateSelfSignedTestCert()
	if err != nil {
		return nil, fmt.Errorf("self-signed test cert: %w", err)
	}
	return &SelfSignedTestTLSConfig{
		Config: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
	}, nil
}

func generateSelfSignedTestCert() (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{Organization: []string{"kubo test"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		DNSNames:     []string{"localhost", "*.libp2p.direct"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  priv,
	}, nil
}
