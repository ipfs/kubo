package cli

import (
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearTelemetryEnv keeps a developer machine that opted out of telemetry from
// changing what these tests exercise.
func clearTelemetryEnv(node *harness.Node) {
	node.Runner.Env["IPFS_TELEMETRY"] = ""
	node.Runner.Env["DO_NOT_TRACK"] = ""
}

func TestTelemetry(t *testing.T) {
	t.Parallel()

	t.Run("opt-out via environment variable", func(t *testing.T) {
		t.Parallel()

		// Create a new node
		node := harness.NewT(t).NewNode().Init()
		node.SetIPFSConfig("Plugins.Plugins.telemetry.Disabled", false)

		// Set the opt-out environment variable
		node.Runner.Env["IPFS_TELEMETRY"] = "off"
		node.Runner.Env["GOLOG_LOG_LEVEL"] = "telemetry=debug"

		// Capture daemon output
		stdout := &harness.Buffer{}
		stderr := &harness.Buffer{}

		// Start daemon with output capture
		node.StartDaemonWithReq(harness.RunRequest{
			CmdOpts: []harness.CmdOpt{
				harness.RunWithStdout(stdout),
				harness.RunWithStderr(stderr),
			},
		}, "")

		time.Sleep(500 * time.Millisecond)

		// Stop daemon
		node.StopDaemon()

		// Verify UUID file was not created or was removed
		uuidPath := filepath.Join(node.Dir, "telemetry_uuid")
		_, err := os.Stat(uuidPath)
		assert.True(t, os.IsNotExist(err), "UUID file should not exist when opted out")
	})

	t.Run("opt-out via config", func(t *testing.T) {
		t.Parallel()

		// Create a new node
		node := harness.NewT(t).NewNode().Init()
		node.SetIPFSConfig("Plugins.Plugins.telemetry.Disabled", false)

		// Set opt-out via config
		node.IPFS("config", "Plugins.Plugins.telemetry.Config.Mode", "off")

		// Enable debug logging
		node.Runner.Env["GOLOG_LOG_LEVEL"] = "telemetry=debug"

		// Capture daemon output
		stdout := &harness.Buffer{}
		stderr := &harness.Buffer{}

		// Start daemon with output capture
		node.StartDaemonWithReq(harness.RunRequest{
			CmdOpts: []harness.CmdOpt{
				harness.RunWithStdout(stdout),
				harness.RunWithStderr(stderr),
			},
		}, "")

		time.Sleep(500 * time.Millisecond)

		// Get daemon output
		output := stdout.String() + stderr.String()

		// Check that telemetry collection is skipped
		assert.Contains(t, output, "telemetry collection skipped: opted out", "Expected telemetry skipped message")

		// Stop daemon
		node.StopDaemon()

		// Verify UUID file was not created or was removed
		uuidPath := filepath.Join(node.Dir, "telemetry_uuid")
		_, err := os.Stat(uuidPath)
		assert.True(t, os.IsNotExist(err), "UUID file should not exist when opted out")
	})

	t.Run("opt-out removes existing UUID file", func(t *testing.T) {
		t.Parallel()

		// Create a new node
		node := harness.NewT(t).NewNode().Init()
		node.SetIPFSConfig("Plugins.Plugins.telemetry.Disabled", false)

		// Create a UUID file manually to simulate previous telemetry run
		uuidPath := filepath.Join(node.Dir, "telemetry_uuid")
		testUUID := "test-uuid-12345"
		err := os.WriteFile(uuidPath, []byte(testUUID), 0600)
		require.NoError(t, err, "Failed to create test UUID file")

		// Verify file exists
		_, err = os.Stat(uuidPath)
		require.NoError(t, err, "UUID file should exist before opt-out")

		// Set the opt-out environment variable
		node.Runner.Env["IPFS_TELEMETRY"] = "off"
		node.Runner.Env["GOLOG_LOG_LEVEL"] = "telemetry=debug"

		// Capture daemon output
		stdout := &harness.Buffer{}
		stderr := &harness.Buffer{}

		// Start daemon with output capture
		node.StartDaemonWithReq(harness.RunRequest{
			CmdOpts: []harness.CmdOpt{
				harness.RunWithStdout(stdout),
				harness.RunWithStderr(stderr),
			},
		}, "")

		time.Sleep(500 * time.Millisecond)

		// Get daemon output
		output := stdout.String() + stderr.String()

		// Check that UUID file was removed
		assert.Contains(t, output, "removed existing telemetry UUID file due to opt-out", "Expected UUID removal message")

		// Stop daemon
		node.StopDaemon()

		// Verify UUID file was removed
		_, err = os.Stat(uuidPath)
		assert.True(t, os.IsNotExist(err), "UUID file should be removed after opt-out")
	})

	t.Run("opt-out via DO_NOT_TRACK", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init()
		node.SetIPFSConfig("Plugins.Plugins.telemetry.Disabled", false)

		node.Runner.Env["DO_NOT_TRACK"] = "1"
		node.Runner.Env["GOLOG_LOG_LEVEL"] = "telemetry=debug"

		// Capture daemon output
		stdout := &harness.Buffer{}
		stderr := &harness.Buffer{}

		node.StartDaemonWithReq(harness.RunRequest{
			CmdOpts: []harness.CmdOpt{
				harness.RunWithStdout(stdout),
				harness.RunWithStderr(stderr),
			},
		}, "")

		time.Sleep(500 * time.Millisecond)

		output := stdout.String() + stderr.String()
		assert.Contains(t, output, "mode set to off by DO_NOT_TRACK", "Expected DO_NOT_TRACK to disable telemetry")
		assert.NotContains(t, output, "Anonymous telemetry will be enabled", "Notice should not be shown when opted out")

		node.StopDaemon()

		uuidPath := filepath.Join(node.Dir, "telemetry_uuid")
		_, err := os.Stat(uuidPath)
		assert.True(t, os.IsNotExist(err), "UUID file should not exist when opted out")
	})

	t.Run("IPFS_TELEMETRY overrides DO_NOT_TRACK", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init()
		node.SetIPFSConfig("Plugins.Plugins.telemetry.Disabled", false)

		// The Kubo-specific variable is the more specific signal, so it wins.
		node.Runner.Env["DO_NOT_TRACK"] = "1"
		node.Runner.Env["IPFS_TELEMETRY"] = "on"
		node.Runner.Env["GOLOG_LOG_LEVEL"] = "telemetry=debug"
		node.IPFS("config", "Plugins.Plugins.telemetry.Config.Endpoint", "https://telemetry.example.com")

		// Capture daemon output
		stdout := &harness.Buffer{}
		stderr := &harness.Buffer{}

		node.StartDaemonWithReq(harness.RunRequest{
			CmdOpts: []harness.CmdOpt{
				harness.RunWithStdout(stdout),
				harness.RunWithStderr(stderr),
			},
		}, "")

		time.Sleep(500 * time.Millisecond)

		output := stdout.String() + stderr.String()
		assert.Contains(t, output, "Anonymous telemetry will be enabled", "Expected telemetry to stay on")

		node.StopDaemon()
	})

	t.Run("enabled by default shows info message", func(t *testing.T) {
		t.Parallel()

		// Create a new node and re-enable the plugin (the harness disables it).
		// Leave everything else at defaults: telemetry is on and reports to the
		// built-in endpoint. Nothing is sent during this test, the first
		// collection is 15 minutes out.
		node := harness.NewT(t).NewNode().Init()
		node.SetIPFSConfig("Plugins.Plugins.telemetry.Disabled", false)
		clearTelemetryEnv(node)

		// Capture daemon output
		stdout := &harness.Buffer{}
		stderr := &harness.Buffer{}

		node.StartDaemonWithReq(harness.RunRequest{
			CmdOpts: []harness.CmdOpt{
				harness.RunWithStdout(stdout),
				harness.RunWithStderr(stderr),
			},
		}, "")

		time.Sleep(500 * time.Millisecond)

		// Get daemon output
		output := stdout.String() + stderr.String()

		// First run: the notice explains what happens and how to opt out.
		assert.Contains(t, output, "Anonymous telemetry")
		assert.Contains(t, output, "https://telemetry.ipshipyard.dev", "Expected the built-in endpoint in the notice")
		assert.Contains(t, output, "No data sent yet", "Expected no data sent message")
		assert.Contains(t, output, "To opt-out before collection starts", "Expected opt-out instructions")
		assert.Contains(t, output, "IPFS_TELEMETRY=off", "Expected the Kubo opt-out in the notice")
		assert.Contains(t, output, "DO_NOT_TRACK=1", "Expected the cross-tool opt-out in the notice")
		assert.Contains(t, output, "Learn more:", "Expected learn more link")

		// Stop daemon
		node.StopDaemon()

		// Verify UUID file was created
		uuidPath := filepath.Join(node.Dir, "telemetry_uuid")
		_, err := os.Stat(uuidPath)
		assert.NoError(t, err, "UUID file should exist when daemon started without telemetry opt-out")
	})

	t.Run("endpoint answering 410 Gone stops telemetry for good", func(t *testing.T) {
		t.Parallel()

		// A collector retires itself with 410 Gone. Nodes pointed at it stop
		// sending, and stay stopped across restarts.
		var requests atomic.Int32
		retiredServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			w.WriteHeader(http.StatusGone)
		}))
		defer retiredServer.Close()

		node := harness.NewT(t).NewNode().Init()
		node.SetIPFSConfig("Plugins.Plugins.telemetry.Disabled", false)
		node.IPFS("config", "Plugins.Plugins.telemetry.Config.Delay", "100ms")
		node.IPFS("config", "Plugins.Plugins.telemetry.Config.Endpoint", retiredServer.URL)
		node.Runner.Env["GOLOG_LOG_LEVEL"] = "telemetry=debug"
		clearTelemetryEnv(node)

		node.StartDaemon()
		require.Eventually(t, func() bool {
			return requests.Load() >= 1
		}, 10*time.Second, 100*time.Millisecond, "expected one report before the endpoint retired")

		retiredPath := filepath.Join(node.Dir, "telemetry_retired")
		require.Eventually(t, func() bool {
			_, err := os.Stat(retiredPath)
			return err == nil
		}, 10*time.Second, 100*time.Millisecond, "expected the retired marker to be written")

		// The identifier is dropped along with the endpoint.
		_, err := os.Stat(filepath.Join(node.Dir, "telemetry_uuid"))
		assert.True(t, os.IsNotExist(err), "UUID file should be removed once the endpoint retires")

		node.StopDaemon()

		// A restart must not resume reporting to that endpoint.
		requests.Store(0)
		node.StartDaemon()
		time.Sleep(2 * time.Second)
		node.StopDaemon()
		assert.Zero(t, requests.Load(), "no further reports should be sent to a retired endpoint")
	})

	t.Run("telemetry schema regression guard", func(t *testing.T) {
		t.Parallel()

		// Define the exact set of expected telemetry fields
		// This list must be updated whenever telemetry fields change
		expectedFields := []string{
			"uuid",
			"agent_version",
			"private_network",
			"bootstrappers_custom",
			"repo_size_bucket",
			"uptime_bucket",
			"reprovider_strategy",
			"provide_dht_sweep_enabled",
			"provide_dht_interval_custom",
			"provide_dht_max_workers_custom",
			"routing_type",
			"routing_accelerated_dht_client",
			"routing_delegated_count",
			"autonat_service_mode",
			"autonat_reachability",
			"swarm_enable_hole_punching",
			"swarm_circuit_addresses",
			"swarm_ipv4_public_addresses",
			"swarm_ipv6_public_addresses",
			"auto_tls_auto_wss",
			"auto_tls_domain_suffix_custom",
			"autoconf",
			"autoconf_custom",
			"discovery_mdns_enabled",
			"platform_os",
			"platform_arch",
			"platform_containerized",
			"platform_vm",
		}

		// Channel to receive captured telemetry data
		telemetryChan := make(chan map[string]any, 1)

		// Create a mock HTTP server to capture telemetry
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Failed to read body", http.StatusBadRequest)
				return
			}

			var telemetryData map[string]any
			if err := json.Unmarshal(body, &telemetryData); err != nil {
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}

			// Send captured data through channel
			select {
			case telemetryChan <- telemetryData:
			default:
			}

			w.WriteHeader(http.StatusOK)
		}))
		defer mockServer.Close()

		// Create a new node
		node := harness.NewT(t).NewNode().Init()
		node.SetIPFSConfig("Plugins.Plugins.telemetry.Disabled", false)

		// Send to the mock endpoint instead of the built-in one, right away
		// instead of 15 minutes in.
		node.IPFS("config", "Plugins.Plugins.telemetry.Config.Mode", "on")
		node.IPFS("config", "Plugins.Plugins.telemetry.Config.Delay", "100ms")
		node.IPFS("config", "Plugins.Plugins.telemetry.Config.Endpoint", mockServer.URL)

		// Enable debug logging to see what's being sent
		node.Runner.Env["GOLOG_LOG_LEVEL"] = "telemetry=debug"

		// Start daemon
		node.StartDaemon()
		defer node.StopDaemon()

		// Wait for telemetry to be sent (configured delay + buffer)
		select {
		case telemetryData := <-telemetryChan:
			receivedFields := slices.Collect(maps.Keys(telemetryData))
			slices.Sort(expectedFields)
			slices.Sort(receivedFields)

			// Fast path: check if fields match exactly
			if !slices.Equal(expectedFields, receivedFields) {
				var missingFields, unexpectedFields []string
				for _, field := range expectedFields {
					if _, ok := telemetryData[field]; !ok {
						missingFields = append(missingFields, field)
					}
				}

				expectedSet := make(map[string]struct{}, len(expectedFields))
				for _, f := range expectedFields {
					expectedSet[f] = struct{}{}
				}
				for field := range telemetryData {
					if _, ok := expectedSet[field]; !ok {
						unexpectedFields = append(unexpectedFields, field)
					}
				}

				t.Fatalf("Telemetry field mismatch:\n"+
					"  Missing fields: %v\n"+
					"  Unexpected fields: %v\n"+
					"  Note: Update expectedFields list in this test when adding/removing telemetry fields",
					missingFields, unexpectedFields)
			}

			t.Logf("Telemetry field validation passed: %d fields verified", len(expectedFields))

		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for telemetry data to be sent")
		}
	})
}
