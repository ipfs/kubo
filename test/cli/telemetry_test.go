package cli

import (
	"os"
	"path/filepath"
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
}
