package autotls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"

	pebbleCA "github.com/letsencrypt/pebble/v2/ca"
	pebbleDB "github.com/letsencrypt/pebble/v2/db"
	pebbleVA "github.com/letsencrypt/pebble/v2/va"
	pebbleWFE "github.com/letsencrypt/pebble/v2/wfe"

	p2pforge "github.com/ipshipyard/p2p-forge/client"

	// Load CoreDNS + p2p-forge plugins (ipparser, acme).
	_ "github.com/ipshipyard/p2p-forge/plugins"
)

// Stack is an in-process pairing of Pebble (ACME server) and p2p-forge
// (registration HTTP endpoint + DNS server). It exists so the AutoTLS E2E
// canary can drive a real kubo daemon through the full cert-issuance flow
// without touching the public internet.
//
// Lifetimes are owned by the test: call Close (registered via t.Cleanup)
// when done.
type Stack struct {
	// ForgeRegistrationEndpoint goes into AutoTLS.RegistrationEndpoint.
	// The URL carries the production-shaped hostname (so the PeerID-auth
	// signature scope matches what the forge server checks) plus the
	// ?dial=/?dns= overrides that point the daemon at our loopback
	// listeners. See docs/config.md for the full syntax.
	ForgeRegistrationEndpoint string

	// ACMEEndpoint is Pebble's directory URL, for AutoTLS.CAEndpoint.
	ACMEEndpoint string

	// PebbleCAPEM is the trust anchor for Pebble's HTTPS listener, for
	// AutoTLS.TrustedCARootsPEM. Different from the issuance root below;
	// kubo uses this only to authenticate the ACME directory connection.
	PebbleCAPEM string

	// PebbleIssuanceRootPEM is the trust anchor for the certs Pebble
	// issues. The canary's HTTPS client uses it to verify the AutoTLS
	// cert kubo serves on /tls/http.
	PebbleIssuanceRootPEM string

	// ForgeDomain is the suffix p2p-forge issues certs under. The canary
	// uses a private .test suffix to avoid colliding with public DNS.
	ForgeDomain string

	// ForgeAuthToken is the value of the registration endpoint's bearer
	// token, also published to the forge process via p2pforge.ForgeAuthEnv.
	ForgeAuthToken string

	close func() // shuts down both servers; idempotent via Stack.Close.
}

// Close shuts down both servers. Safe to call multiple times.
func (s *Stack) Close() {
	if s.close != nil {
		s.close()
		s.close = nil
	}
}

// NewStack stands up Pebble and p2p-forge on free local ports. Returns a
// Stack populated with the endpoint URLs and trust material the kubo
// daemon needs to talk to them. The setup mirrors the canonical pattern
// used in p2p-forge's own end-to-end test (e2e_test.go in that repo) so a
// regression in either layer surfaces here.
func NewStack(t *testing.T) *Stack {
	t.Helper()

	const (
		forgeDomain    = "libp2p.test"
		forgeRegHost   = "registration.libp2p.test"
		forgeAuthToken = "test-token"
	)

	// p2p-forge reads its auth token from this env var. Pebble skips its
	// random 0-15s VA sleep when PEBBLE_VA_NOSLEEP is set; without that
	// the canary spends most of its budget idle. We restore both on
	// cleanup so other parallel tests don't observe a leak.
	prevAuth, hadAuth := setEnv(t, p2pforge.ForgeAuthEnv, forgeAuthToken)
	prevNoSleep, hadNoSleep := setEnv(t, "PEBBLE_VA_NOSLEEP", "1")

	// Reserve a port for the forge HTTP registration endpoint. The
	// acme plugin will bind it a moment later; reserving up-front lets
	// the Corefile reference a known port.
	tmpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve forge http port: %v", err)
	}
	httpPort := tmpListener.Addr().(*net.TCPAddr).Port
	_ = tmpListener.Close()

	// Reserve one port for DNS so CoreDNS binds the same port on UDP and
	// TCP. With .:0 the OS hands each protocol an independent random
	// port, and Pebble's VA fails when it retries TXT lookups over TCP.
	tmpUDP, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve forge dns port: %v", err)
	}
	dnsPort := tmpUDP.LocalAddr().(*net.UDPAddr).Port
	_ = tmpUDP.Close()

	// Configure the CoreDNS instance p2p-forge runs on. The ipparser
	// plugin resolves <peerid>.<forgeDomain> from the embedded IP; the
	// acme plugin owns the DNS-01 TXT records and the HTTP registration
	// endpoint.
	tmpDir := t.TempDir()
	dnsserver.Directives = []string{"log", "whoami", "startup", "shutdown", "ipparser", "acme"}
	corefile := fmt.Sprintf(`.:%[5]d {
		log
		ipparser %[1]s
		acme %[1]s {
			registration-domain %[2]s listen-address=:%[3]d external-tls=true
			database-type badger %[4]s
		}
	}`, forgeDomain, forgeRegHost, httpPort, tmpDir, dnsPort)
	dnsInstance, err := caddy.Start(&corefileInput{body: []byte(corefile)})
	if err != nil {
		t.Fatalf("start p2p-forge (caddy): %v", err)
	}
	// dnsAddr is what Pebble's VA and the kubo daemon's pre-flight check
	// both dial; same port on UDP and TCP (see reservation above).
	dnsAddr := fmt.Sprintf("127.0.0.1:%d", dnsPort)

	// Stand up Pebble. The VA needs the forge DNS server's address so it
	// can resolve the DNS-01 TXT records p2p-forge publishes.
	pebbleLogger := log.New(os.Stderr, "pebble: ", log.LstdFlags)
	db := pebbleDB.NewMemoryStore()
	// ocspResponderURL="" (none), keyAlg="ecdsa" (Pebble panics on
	// empty), 1 alternate root, chain length 1 (Pebble requires at
	// least one intermediate). One issuance profile so wfe.NewOrder's
	// random-profile pick doesn't Intn(0)-panic.
	profiles := map[string]pebbleCA.Profile{
		"shortlived": {
			Description:    "Pebble test profile",
			ValidityPeriod: 7 * 24 * 60 * 60, // seconds
		},
	}
	ca := pebbleCA.New(pebbleLogger, db, "", "ecdsa", 1, 1, profiles)
	va := pebbleVA.New(pebbleLogger, 0, 0, false, dnsAddr, db)
	// nil caaIdentities skips CAA checks (we have no DNS CAA records for
	// libp2p.test anyway). strict=false, requireEAB=false, retryAfter
	// values match Pebble's defaults.
	wfe := pebbleWFE.New(pebbleLogger, db, va, ca, nil, false, false, 3, 5)

	// Pebble's ACME endpoint must speak HTTPS. The self-signed cert
	// below is what AutoTLS.TrustedCARootsPEM teaches kubo's certmagic
	// to trust.
	pebbleCertPEM, pebbleKeyPEM, err := generateLoopbackCert("127.0.0.1")
	if err != nil {
		t.Fatalf("pebble self-signed cert: %v", err)
	}
	pebbleCert, err := tls.X509KeyPair(pebbleCertPEM, pebbleKeyPEM)
	if err != nil {
		t.Fatalf("load pebble cert: %v", err)
	}
	acmeListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen pebble: %v", err)
	}
	tlsListener := tls.NewListener(acmeListener, &tls.Config{Certificates: []tls.Certificate{pebbleCert}})
	acmeServer := &http.Server{Handler: wfe.Handler()}
	go func() { _ = acmeServer.Serve(tlsListener) }()

	stack := &Stack{
		ForgeRegistrationEndpoint: fmt.Sprintf("http://%s/?dial=127.0.0.1:%d&dns=%s", forgeRegHost, httpPort, dnsAddr),
		ACMEEndpoint:              fmt.Sprintf("https://%s%s", acmeListener.Addr(), pebbleWFE.DirectoryPath),
		PebbleCAPEM:               string(pebbleCertPEM),
		PebbleIssuanceRootPEM:     string(ca.GetRootCert(0).PEM()),
		ForgeDomain:               forgeDomain,
		ForgeAuthToken:            forgeAuthToken,
		close: func() {
			_ = acmeServer.Close()
			_ = dnsInstance.Stop()
			dnsInstance.Wait()
			restoreEnv(p2pforge.ForgeAuthEnv, prevAuth, hadAuth)
			restoreEnv("PEBBLE_VA_NOSLEEP", prevNoSleep, hadNoSleep)
		},
	}
	t.Cleanup(stack.Close)
	return stack
}

// setEnv sets key=val for the duration of the test, returning the previous
// value (if any) so a paired restoreEnv call can put it back on cleanup. We
// roll our own instead of t.Setenv because the canary uses t.Parallel.
func setEnv(t *testing.T, key, val string) (prev string, had bool) {
	t.Helper()
	prev, had = os.LookupEnv(key)
	if err := os.Setenv(key, val); err != nil {
		t.Fatalf("set %s: %v", key, err)
	}
	return prev, had
}

// restoreEnv puts an environment variable back to the value setEnv saw.
func restoreEnv(key, prev string, had bool) {
	if had {
		_ = os.Setenv(key, prev)
	} else {
		_ = os.Unsetenv(key)
	}
}

// generateLoopbackCert produces a self-signed cert + private key covering
// the supplied IP. Used for Pebble's HTTPS listener; kubo's certmagic
// trusts this cert via AutoTLS.TrustedCARootsPEM.
func generateLoopbackCert(ipAddr string) ([]byte, []byte, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{Organization: []string{"kubo autotls e2e"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP(ipAddr)},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM, nil
}

// corefileInput implements the caddy.Input interface around a raw Corefile.
type corefileInput struct{ body []byte }

func (i *corefileInput) Body() []byte       { return i.body }
func (i *corefileInput) Path() string       { return "Corefile" }
func (i *corefileInput) ServerType() string { return "dns" }
