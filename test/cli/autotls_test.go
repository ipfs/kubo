package cli

import (
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
