package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/require"
)

// autotlsPostponedMsg is the marker of the ERROR the p2p-forge client logs
// when its pre-issuance broker health check fails and certificate setup is
// postponed until the broker recovers.
const autotlsPostponedMsg = "certificate setup postponed"

// wssWildcardFragment appears in swarm listen addrs only when the AutoTLS
// machinery was wired up (AutoWSS appended the wildcard WSS listener).
const wssWildcardFragment = "/tls/sni/"

// unroutableURL is guaranteed to refuse connections without depending on the
// state of any real port (port 0 is never connectable).
const unroutableURL = "http://127.0.0.1:0"

// countingBroker is a fake p2p-forge broker that records /v1/health probes.
func countingBroker(t *testing.T, status int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var probes atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/health" {
			probes.Add(1)
			w.WriteHeader(status)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv, &probes
}

// countingCA is a fake ACME server that records requests for its directory,
// the first thing any ACME client asks for. It answers with an error, so
// nothing is ever issued; the count is only evidence that the daemon went to a
// CA on its own.
func countingCA(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var directoryFetches atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dir" {
			directoryFetches.Add(1)
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv, &directoryFetches
}

// autotlsNode inits a node with AutoTLS explicitly enabled against the given
// broker URL. Forced public reachability plus an announced public address
// satisfy the conditions the p2p-forge client waits for before it contacts
// the broker, so the pre-issuance health check can be observed in an
// isolated test. CAEndpoint points at an unroutable address so no test can
// ever reach a real ACME CA, even when issuance starts. GOLOG env is pinned
// so stderr assertions do not depend on ambient GOLOG_* variables.
func autotlsNode(t *testing.T, brokerURL string) *harness.Node {
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.Runner.Env["GOLOG_LOG_LEVEL"] = "error"
	node.Runner.Env["GOLOG_OUTPUT"] = "stderr"
	node.UpdateConfig(func(cfg *config.Config) {
		cfg.AutoTLS.Enabled = config.True
		cfg.AutoTLS.RegistrationEndpoint = config.NewOptionalString(brokerURL)
		cfg.AutoTLS.CAEndpoint = config.NewOptionalString(unroutableURL)
		cfg.Internal.Libp2pForceReachability = config.NewOptionalString("public")
		cfg.Addresses.Announce = []string{"/ip4/1.2.3.4/tcp/4001"}
	})
	return node
}

func TestAutoTLSBrokerHealthCheck(t *testing.T) {
	t.Parallel()

	t.Run("registration delay defers any broker traffic", func(t *testing.T) {
		t.Parallel()
		broker, probes := countingBroker(t, http.StatusNoContent)

		node := autotlsNode(t, broker.URL)
		node.UpdateConfig(func(cfg *config.Config) {
			// implicit enable: the default 1h registration delay applies, so
			// an ephemeral node like this one must produce no broker traffic
			// at all, even while it looks publicly reachable
			cfg.AutoTLS.Enabled = config.Default
		})
		node.StartDaemon()
		defer node.StopDaemon()

		require.Never(t, func() bool { return probes.Load() > 0 }, 2*time.Second, 100*time.Millisecond,
			"daemon must not contact broker before the registration delay")
		require.NotContains(t, node.Daemon.Stderr.String(), autotlsPostponedMsg)
	})

	t.Run("postpones issuance while broker is unhealthy", func(t *testing.T) {
		t.Parallel()
		broker, probes := countingBroker(t, http.StatusServiceUnavailable)

		node := autotlsNode(t, broker.URL)
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.AutoTLS.RegistrationDelay = config.NewOptionalDuration(1 * time.Second)
		})
		node.StartDaemon()
		defer node.StopDaemon()

		require.True(t, waitForLogMessage(node.Daemon.Stderr, autotlsPostponedMsg, 15*time.Second),
			"p2p-forge client should postpone certificate setup when broker is unhealthy")
		require.GreaterOrEqual(t, probes.Load(), int32(1))
	})

	t.Run("proceeds with issuance when broker is healthy", func(t *testing.T) {
		t.Parallel()
		broker, probes := countingBroker(t, http.StatusNoContent)

		node := autotlsNode(t, broker.URL)
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.AutoTLS.RegistrationDelay = config.NewOptionalDuration(1 * time.Second)
		})
		node.StartDaemon()
		defer node.StopDaemon()

		require.Eventually(t, func() bool { return probes.Load() >= 1 }, 15*time.Second, 100*time.Millisecond,
			"p2p-forge client should probe broker health before issuance")
		require.NotContains(t, node.Daemon.Stderr.String(), autotlsPostponedMsg)

		// AutoTLS machinery is wired up: AutoWSS added the wildcard listener
		listenAddrs := node.IPFS("swarm", "addrs", "listen").Stdout.String()
		require.Contains(t, listenAddrs, wssWildcardFragment)
	})

	t.Run("goes to the CA for its own address instead of the broker", func(t *testing.T) {
		t.Parallel()
		broker, probes := countingBroker(t, http.StatusNoContent)
		ca, directoryFetches := countingCA(t)

		// A node that listens on the port the certificate authority validates
		// on asks the authority to certify its address, and registers no name
		// with anyone. It stays that way even when issuance fails, which it
		// does here: the authority is a stub that answers every request with an
		// error.
		//
		// Port 443 needs privileges no test has, so the node listens on a free
		// port and AutoTLS.IPCertsPort points the flow at it. The registration
		// delay is pinned to zero: it gates first-time issuance, so leaving it
		// at the default would make "no broker traffic" true for an hour no
		// matter what this code does.
		port := harness.NewRandPort()
		node := autotlsNode(t, broker.URL)
		node.UpdateConfig(func(cfg *config.Config) {
			// A wildcard bind, like a real node on 443: a listener on loopback
			// is not somewhere a certificate authority can reach.
			cfg.Addresses.Swarm = []string{fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port)}
			cfg.Addresses.Announce = []string{fmt.Sprintf("/ip4/1.2.3.4/tcp/%d", port)}
			cfg.AutoTLS.IPCertsPort = config.NewOptionalInteger(int64(port))
			cfg.AutoTLS.CAEndpoint = config.NewOptionalString(ca.URL + "/dir")
			cfg.AutoTLS.RegistrationDelay = config.NewOptionalDuration(0)
		})
		node.StartDaemon()
		defer node.StopDaemon()

		// Reaching the CA is what says the node tried to certify its own
		// address. The broker path would reach the CA too, but only after the
		// health check below, which never happens here.
		require.Eventually(t, func() bool { return directoryFetches.Load() > 0 }, 30*time.Second, 100*time.Millisecond,
			"daemon should ask the CA to certify its own address")
		require.Zero(t, probes.Load(),
			"daemon registered with the broker even though it could certify its own address")

		// And it stays that way. The certificate authority here never issues
		// anything, and that is not a reason to go and register a name.
		require.Never(t, func() bool { return probes.Load() > 0 }, 3*time.Second, 100*time.Millisecond,
			"daemon registered with the broker after its own certificate failed")
	})

	t.Run("IPCerts off sends a node on port 443 to the broker", func(t *testing.T) {
		t.Parallel()
		broker, probes := countingBroker(t, http.StatusNoContent)
		ca, _ := countingCA(t)

		// The listener alone decides the path, so turning the option off is
		// the only way a node on that port uses the broker.
		port := harness.NewRandPort()
		node := autotlsNode(t, broker.URL)
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Addresses.Swarm = []string{fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port)}
			cfg.Addresses.Announce = []string{fmt.Sprintf("/ip4/1.2.3.4/tcp/%d", port)}
			cfg.AutoTLS.IPCerts = config.False
			cfg.AutoTLS.IPCertsPort = config.NewOptionalInteger(int64(port))
			cfg.AutoTLS.CAEndpoint = config.NewOptionalString(ca.URL + "/dir")
			cfg.AutoTLS.RegistrationDelay = config.NewOptionalDuration(0)
		})
		node.StartDaemon()
		defer node.StopDaemon()

		// Reaching the broker is the signal: a node certifying its own address
		// never does. Both paths talk to a certificate authority eventually,
		// so the CA stub here only keeps the test off the real one.
		require.Eventually(t, func() bool { return probes.Load() > 0 }, 15*time.Second, 100*time.Millisecond,
			"daemon should register with the broker when IP certificates are off")
	})

	t.Run("explicit enable goes through the same health check", func(t *testing.T) {
		t.Parallel()
		broker, probes := countingBroker(t, http.StatusServiceUnavailable)

		// AutoTLS.Enabled=true with no custom delay means a zero registration
		// delay: the health check runs as soon as the node looks publicly
		// reachable, through the same code path as the delayed flow
		node := autotlsNode(t, broker.URL)
		node.StartDaemon()
		defer node.StopDaemon()

		require.True(t, waitForLogMessage(node.Daemon.Stderr, autotlsPostponedMsg, 15*time.Second),
			"explicitly enabled AutoTLS should postpone certificate setup when broker is unhealthy")
		require.GreaterOrEqual(t, probes.Load(), int32(1))
	})
}
