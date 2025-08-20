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

		// Check that telemetry is disabled
		assert.Contains(t, output, "telemetry disabled via opt-out", "Expected telemetry disabled message")

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

		// Check that telemetry is disabled
		assert.Contains(t, output, "telemetry disabled via opt-out", "Expected telemetry disabled message")
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

	t.Run("telemetry enabled shows info message", func(t *testing.T) {
		t.Parallel()

		// Create a new node
		node := harness.NewT(t).NewNode().Init()

		// Capture daemon output
		stdout := &harness.Buffer{}
		stderr := &harness.Buffer{}

		// Don't set opt-out, so telemetry will be enabled
		// This should trigger the info message on first run
		node.StartDaemonWithReq(harness.RunRequest{
			CmdOpts: []harness.CmdOpt{
				harness.RunWithStdout(stdout),
				harness.RunWithStderr(stderr),
			},
		}, "")

		time.Sleep(500 * time.Millisecond)

		// Get daemon output
		output := stdout.String() + stderr.String()

		// First run - should show info message
		assert.Contains(t, output, "Anonymous telemetry")
		assert.Contains(t, output, "No data sent yet", "Expected no data sent message")
		assert.Contains(t, output, "To opt-out before collection starts", "Expected opt-out instructions")
		assert.Contains(t, output, "Learn more:", "Expected learn more link")

		// Stop daemon
		node.StopDaemon()

		// Verify UUID file was created
		uuidPath := filepath.Join(node.Dir, "telemetry_uuid")
		_, err := os.Stat(uuidPath)
		assert.NoError(t, err, "UUID file should exist when daemon started without telemetry opt-out")
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
		telemetryChan := make(chan map[string]interface{}, 1)

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

			var telemetryData map[string]interface{}
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

		// Configure telemetry with a very short delay for testing
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
