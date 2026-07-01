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
	"testing"
	"time"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	t.Run("disabled by default (opt-in)", func(t *testing.T) {
		t.Parallel()

		// Create a new node and re-enable the plugin (the harness disables it).
		// Leave Mode unset so we exercise the default, which is off.
		node := harness.NewT(t).NewNode().Init()
		node.SetIPFSConfig("Plugins.Plugins.telemetry.Disabled", false)
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

		// No opt-in: no info message and no data collection.
		assert.Contains(t, output, "telemetry not enabled (opt-in)", "Expected opt-in disabled message")
		assert.NotContains(t, output, "Telemetry is enabled", "Info message should not be shown when telemetry is off by default")

		node.StopDaemon()

		// Verify UUID file was not created
		uuidPath := filepath.Join(node.Dir, "telemetry_uuid")
		_, err := os.Stat(uuidPath)
		assert.True(t, os.IsNotExist(err), "UUID file should not exist when telemetry is off by default")
	})

	t.Run("default leaves existing UUID file untouched", func(t *testing.T) {
		t.Parallel()

		// The implicit default (no env, no config Mode) must do no work, not
		// even disk IO: an existing UUID file is left in place, only an
		// explicit "off" removes it.
		node := harness.NewT(t).NewNode().Init()
		node.SetIPFSConfig("Plugins.Plugins.telemetry.Disabled", false)
		node.Runner.Env["GOLOG_LOG_LEVEL"] = "telemetry=debug"

		uuidPath := filepath.Join(node.Dir, "telemetry_uuid")
		require.NoError(t, os.WriteFile(uuidPath, []byte("existing-uuid"), 0600))

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

		assert.Contains(t, output, "telemetry not enabled (opt-in)", "Expected opt-in disabled message")
		assert.NotContains(t, output, "removed existing telemetry UUID file", "Default must not touch the UUID file")

		node.StopDaemon()

		// The file must still be there and unchanged.
		data, err := os.ReadFile(uuidPath)
		require.NoError(t, err, "UUID file should still exist under the implicit default")
		assert.Equal(t, "existing-uuid", string(data), "UUID file contents should be unchanged")
	})

	t.Run("opt-in shows info message", func(t *testing.T) {
		t.Parallel()

		// Create a new node
		node := harness.NewT(t).NewNode().Init()
		node.SetIPFSConfig("Plugins.Plugins.telemetry.Disabled", false)

		// Opt in: telemetry is off by default, so enable it and point it at an
		// endpoint. "on" logs the startup notice once on the first run.
		node.IPFS("config", "Plugins.Plugins.telemetry.Config.Mode", "on")
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

		// Get daemon output
		output := stdout.String() + stderr.String()

		// Opt-in run should show the info message
		assert.Contains(t, output, "Telemetry is enabled", "Expected telemetry enabled message")
		assert.Contains(t, output, "To disable telemetry", "Expected disable instructions")
		assert.Contains(t, output, "Learn more:", "Expected learn more link")

		// Stop daemon
		node.StopDaemon()

		// Verify UUID file was created
		uuidPath := filepath.Join(node.Dir, "telemetry_uuid")
		_, err := os.Stat(uuidPath)
		assert.NoError(t, err, "UUID file should exist when telemetry is opted in")
	})

	t.Run("auto is treated as off", func(t *testing.T) {
		t.Parallel()

		// Create a new node
		node := harness.NewT(t).NewNode().Init()
		node.SetIPFSConfig("Plugins.Plugins.telemetry.Disabled", false)

		// "auto" is a legacy value; it now behaves like the default (off), even
		// with an endpoint set.
		node.IPFS("config", "Plugins.Plugins.telemetry.Config.Mode", "auto")
		node.IPFS("config", "Plugins.Plugins.telemetry.Config.Endpoint", "https://telemetry.example.com")
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

		// auto must stay off: no info message and no data collection.
		assert.Contains(t, output, "telemetry not enabled (opt-in)", "auto should behave like off")
		assert.NotContains(t, output, "Telemetry is enabled", "auto must not show the opt-in banner")

		node.StopDaemon()

		// Verify UUID file was not created
		uuidPath := filepath.Join(node.Dir, "telemetry_uuid")
		_, err := os.Stat(uuidPath)
		assert.True(t, os.IsNotExist(err), "auto must not create a UUID file")
	})

	t.Run("enabled without endpoint is skipped", func(t *testing.T) {
		t.Parallel()

		// Create a new node
		node := harness.NewT(t).NewNode().Init()
		node.SetIPFSConfig("Plugins.Plugins.telemetry.Disabled", false)

		// Enable telemetry but do not configure an endpoint.
		node.IPFS("config", "Plugins.Plugins.telemetry.Config.Mode", "on")
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

		// Enabled without an endpoint warns and sends nothing.
		assert.Contains(t, output, "no endpoint is configured", "Expected missing-endpoint warning")

		node.StopDaemon()

		// Without an endpoint, no UUID is generated.
		uuidPath := filepath.Join(node.Dir, "telemetry_uuid")
		_, err := os.Stat(uuidPath)
		assert.True(t, os.IsNotExist(err), "UUID file should not be created without an endpoint")
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

		// Opt in to telemetry (off by default) and configure a very short delay
		// and the mock endpoint for testing.
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
