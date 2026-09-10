package libp2p

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// generateTestCert creates a self-signed certificate for the given hostname
// and writes it (with its private key) to certPath and keyPath in PEM format.
func generateTestCert(t *testing.T, host, certPath, keyPath string) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("generate serial: %v", err)
	}

	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		DNSNames:     []string{host},
	}

	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	if err := writePEM(certPath, "CERTIFICATE", der); err != nil {
		t.Fatalf("write cert file: %v", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	if err := writePEM(keyPath, "EC PRIVATE KEY", keyDER); err != nil {
		t.Fatalf("write key file: %v", err)
	}
}

func writePEM(path, blockType string, der []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return pem.Encode(f, &pem.Block{Type: blockType, Bytes: der})
}

func TestManualCertCache_LoadAndCache(t *testing.T) {
	tmp := t.TempDir()
	host := "example.net"
	certPath := filepath.Join(tmp, host+".crt")
	keyPath := filepath.Join(tmp, host+".key")
	generateTestCert(t, host, certPath, keyPath)

	cache := newManualCertCache()

	// First load reads from disk.
	cert, err := cache.load(tmp, host)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	if cert == nil {
		t.Fatal("first load returned nil cert")
	}

	// Second load comes from cache (same mtime).
	cert2, err := cache.load(tmp, host)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if cert2 == nil {
		t.Fatal("second load returned nil cert")
	}

	// Touch the cert file to invalidate cache; next load reads from disk again.
	now := time.Now().Add(time.Second)
	if err := os.Chtimes(certPath, now, now); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	cert3, err := cache.load(tmp, host)
	if err != nil {
		t.Fatalf("third load after touch: %v", err)
	}
	if cert3 == nil {
		t.Fatal("third load returned nil cert")
	}
}

func TestManualCertCache_MissingFiles(t *testing.T) {
	tmp := t.TempDir()
	cache := newManualCertCache()

	_, err := cache.load(tmp, "nonexistent.example")
	if err == nil {
		t.Fatal("expected error for missing cert files")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("expected os.ErrNotExist, got: %v", err)
	}
}

func TestManualCertCache_EmptySNI(t *testing.T) {
	tmp := t.TempDir()
	cache := newManualCertCache()

	cert, err := cache.load(tmp, "")
	if err != nil {
		t.Fatalf("empty SNI should not error: %v", err)
	}
	if cert != nil {
		t.Fatal("empty SNI should return nil cert")
	}
}

func TestManualCertGetter_ManualCertPresent(t *testing.T) {
	tmp := t.TempDir()
	host := "custom.example"
	generateTestCert(t, host, filepath.Join(tmp, host+".crt"), filepath.Join(tmp, host+".key"))

	cache := newManualCertCache()
	called := false
	fallback := func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		called = true
		return nil, nil
	}
	getter := manualCertGetter(tmp, cache, fallback)

	cert, err := getter(&tls.ClientHelloInfo{ServerName: host})
	if err != nil {
		t.Fatalf("getter: %v", err)
	}
	if cert == nil {
		t.Fatal("getter returned nil cert")
	}
	if called {
		t.Fatal("fallback should not be called when manual cert is present")
	}
}

func TestManualCertGetter_FallsBackToForge(t *testing.T) {
	tmp := t.TempDir()
	host := "libp2p.direct" // no manual cert files for this host

	cache := newManualCertCache()
	fallbackCert := &tls.Certificate{}
	called := false
	fallback := func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		called = true
		return fallbackCert, nil
	}
	getter := manualCertGetter(tmp, cache, fallback)

	cert, err := getter(&tls.ClientHelloInfo{ServerName: host})
	if err != nil {
		t.Fatalf("getter: %v", err)
	}
	if cert != fallbackCert {
		t.Fatal("expected fallback cert")
	}
	if !called {
		t.Fatal("fallback should be called when no manual cert exists")
	}
}

func TestManualCertGetter_NoFallbackNoCert(t *testing.T) {
	tmp := t.TempDir()
	host := "nonexistent.example"

	cache := newManualCertCache()
	getter := manualCertGetter(tmp, cache, nil)

	_, err := getter(&tls.ClientHelloInfo{ServerName: host})
	if err == nil {
		t.Fatal("expected error when no manual cert and no fallback")
	}
}
