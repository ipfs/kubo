package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	. "github.com/ipfs/kubo/test/cli/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogLevel(t *testing.T) {

	t.Run("CLI", func(t *testing.T) {
		t.Run("level '*' shows all subsystems", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			expectedSubsystems := getExpectedSubsystems(t, node)

			res := node.IPFS("log", "level", "*")
			assert.NoError(t, res.Err)
			assert.Empty(t, res.Stderr.Lines())

			actualSubsystems := parseCLIOutput(t, res.Stdout.String())

			// Should show all subsystems plus the (default) entry
			assert.GreaterOrEqual(t, len(actualSubsystems), len(expectedSubsystems))

			validateAllSubsystemsPresentCLI(t, expectedSubsystems, actualSubsystems, "CLI output")

			// Should have the (default) entry
			_, hasDefault := actualSubsystems["(default)"]
			assert.True(t, hasDefault, "Should have '(default)' entry")
		})

		t.Run("level 'all' shows all subsystems (alias for '*')", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			expectedSubsystems := getExpectedSubsystems(t, node)

			res := node.IPFS("log", "level", "all")
			assert.NoError(t, res.Err)
			assert.Empty(t, res.Stderr.Lines())

			actualSubsystems := parseCLIOutput(t, res.Stdout.String())

			// Should show all subsystems plus the (default) entry
			assert.GreaterOrEqual(t, len(actualSubsystems), len(expectedSubsystems))

			validateAllSubsystemsPresentCLI(t, expectedSubsystems, actualSubsystems, "CLI output")

			// Should have the (default) entry
			_, hasDefault := actualSubsystems["(default)"]
			assert.True(t, hasDefault, "Should have '(default)' entry")
		})

		t.Run("get level for specific subsystem", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			node.IPFS("log", "level", "core", "debug")
			res := node.IPFS("log", "level", "core")
			assert.NoError(t, res.Err)
			assert.Empty(t, res.Stderr.Lines())

			output := res.Stdout.String()
			lines := SplitLines(output)

			assert.Equal(t, 1, len(lines))

			line := strings.TrimSpace(lines[0])
			assert.Equal(t, "debug", line)
		})

		t.Run("get level with no args returns default level", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			res1 := node.IPFS("log", "level", "*", "fatal")
			assert.NoError(t, res1.Err)
			assert.Empty(t, res1.Stderr.Lines())

			res := node.IPFS("log", "level")
			assert.NoError(t, res.Err)
			assert.Equal(t, 0, len(res.Stderr.Lines()))

			output := res.Stdout.String()
			lines := SplitLines(output)

			assert.Equal(t, 1, len(lines))

			line := strings.TrimSpace(lines[0])
			assert.Equal(t, "fatal", line)
		})

		t.Run("get level reflects runtime log level changes", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
			defer node.StopDaemon()

			node.IPFS("log", "level", "core", "debug")
			res := node.IPFS("log", "level", "core")
			assert.NoError(t, res.Err)

			output := res.Stdout.String()
			lines := SplitLines(output)

			assert.Equal(t, 1, len(lines))

			line := strings.TrimSpace(lines[0])
			assert.Equal(t, "debug", line)
		})

		t.Run("get level with non-existent subsystem returns error", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			res := node.RunIPFS("log", "level", "non-existent-subsystem")
			assert.Error(t, res.Err)
			assert.NotEqual(t, 0, len(res.Stderr.Lines()))
		})

		t.Run("set level to 'default' keyword", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			// First set a specific subsystem to a different level
			res1 := node.IPFS("log", "level", "core", "debug")
			assert.NoError(t, res1.Err)
			assert.Contains(t, res1.Stdout.String(), "Changed log level of 'core' to 'debug'")

			// Verify it was set to debug
			res2 := node.IPFS("log", "level", "core")
			assert.NoError(t, res2.Err)
			assert.Equal(t, "debug", strings.TrimSpace(res2.Stdout.String()))

			// Get the current default level (should be 'error' since unchanged)
			res3 := node.IPFS("log", "level")
			assert.NoError(t, res3.Err)
			defaultLevel := strings.TrimSpace(res3.Stdout.String())
			assert.Equal(t, "error", defaultLevel, "Default level should be 'error' when unchanged")

			// Now set the subsystem back to default
			res4 := node.IPFS("log", "level", "core", "default")
			assert.NoError(t, res4.Err)
			assert.Contains(t, res4.Stdout.String(), "Changed log level of 'core' to")

			// Verify it's now at the default level (should be 'error')
			res5 := node.IPFS("log", "level", "core")
			assert.NoError(t, res5.Err)
			assert.Equal(t, "error", strings.TrimSpace(res5.Stdout.String()))
		})

		t.Run("set all subsystems with 'all' changes default (alias for '*')", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			// Initial state - default should be 'error'
			res := node.IPFS("log", "level")
			assert.NoError(t, res.Err)
			assert.Equal(t, "error", strings.TrimSpace(res.Stdout.String()))

			// Set one subsystem to a different level
			res = node.IPFS("log", "level", "core", "debug")
			assert.NoError(t, res.Err)

			// Default should still be 'error'
			res = node.IPFS("log", "level")
			assert.NoError(t, res.Err)
			assert.Equal(t, "error", strings.TrimSpace(res.Stdout.String()))

			// Now use 'all' to set everything to 'info'
			res = node.IPFS("log", "level", "all", "info")
			assert.NoError(t, res.Err)
			assert.Contains(t, res.Stdout.String(), "Changed log level of '*' to 'info'")

			// Default should now be 'info'
			res = node.IPFS("log", "level")
			assert.NoError(t, res.Err)
			assert.Equal(t, "info", strings.TrimSpace(res.Stdout.String()))

			// Core should also be 'info' (overwritten by 'all')
			res = node.IPFS("log", "level", "core")
			assert.NoError(t, res.Err)
			assert.Equal(t, "info", strings.TrimSpace(res.Stdout.String()))

			// Any other subsystem should also be 'info'
			res = node.IPFS("log", "level", "dht")
			assert.NoError(t, res.Err)
			assert.Equal(t, "info", strings.TrimSpace(res.Stdout.String()))
		})

		t.Run("set all subsystems with '*' changes default", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			// Initial state - default should be 'error'
			res := node.IPFS("log", "level")
			assert.NoError(t, res.Err)
			assert.Equal(t, "error", strings.TrimSpace(res.Stdout.String()))

			// Set one subsystem to a different level
			res = node.IPFS("log", "level", "core", "debug")
			assert.NoError(t, res.Err)

			// Default should still be 'error'
			res = node.IPFS("log", "level")
			assert.NoError(t, res.Err)
			assert.Equal(t, "error", strings.TrimSpace(res.Stdout.String()))

			// Now use '*' to set everything to 'info'
			res = node.IPFS("log", "level", "*", "info")
			assert.NoError(t, res.Err)
			assert.Contains(t, res.Stdout.String(), "Changed log level of '*' to 'info'")

			// Default should now be 'info'
			res = node.IPFS("log", "level")
			assert.NoError(t, res.Err)
			assert.Equal(t, "info", strings.TrimSpace(res.Stdout.String()))

			// Core should also be 'info' (overwritten by '*')
			res = node.IPFS("log", "level", "core")
			assert.NoError(t, res.Err)
			assert.Equal(t, "info", strings.TrimSpace(res.Stdout.String()))

			// Any other subsystem should also be 'info'
			res = node.IPFS("log", "level", "dht")
			assert.NoError(t, res.Err)
			assert.Equal(t, "info", strings.TrimSpace(res.Stdout.String()))
		})

		t.Run("'all' in get mode shows (default) entry (alias for '*')", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			// Get all levels with 'all'
			res := node.IPFS("log", "level", "all")
			assert.NoError(t, res.Err)

			output := res.Stdout.String()

			// Should contain "(default): error" entry
			assert.Contains(t, output, "(default): error", "Should show default level with (default) key")

			// Should also contain various subsystems
			assert.Contains(t, output, "core: error")
			assert.Contains(t, output, "dht: error")
		})

		t.Run("'*' in get mode shows (default) entry", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			// Get all levels with '*'
			res := node.IPFS("log", "level", "*")
			assert.NoError(t, res.Err)

			output := res.Stdout.String()

			// Should contain "(default): error" entry
			assert.Contains(t, output, "(default): error", "Should show default level with (default) key")

			// Should also contain various subsystems
			assert.Contains(t, output, "core: error")
			assert.Contains(t, output, "dht: error")
		})

		t.Run("set all subsystems to 'default' using 'all' (alias for '*')", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			// Get the original default level (just for reference, it should be "error")
			res0 := node.IPFS("log", "level")
			assert.NoError(t, res0.Err)
			assert.Equal(t, "error", strings.TrimSpace(res0.Stdout.String()))

			// First set all subsystems to debug using 'all'
			res1 := node.IPFS("log", "level", "all", "debug")
			assert.NoError(t, res1.Err)
			assert.Contains(t, res1.Stdout.String(), "Changed log level of '*' to 'debug'")

			// Verify a specific subsystem is at debug
			res2 := node.IPFS("log", "level", "core")
			assert.NoError(t, res2.Err)
			assert.Equal(t, "debug", strings.TrimSpace(res2.Stdout.String()))

			// Verify the default level is now debug
			res3 := node.IPFS("log", "level")
			assert.NoError(t, res3.Err)
			assert.Equal(t, "debug", strings.TrimSpace(res3.Stdout.String()))

			// Now set all subsystems back to default (which is now "debug") using 'all'
			res4 := node.IPFS("log", "level", "all", "default")
			assert.NoError(t, res4.Err)
			assert.Contains(t, res4.Stdout.String(), "Changed log level of '*' to")

			// The subsystem should still be at debug (because that's what default is now)
			res5 := node.IPFS("log", "level", "core")
			assert.NoError(t, res5.Err)
			assert.Equal(t, "debug", strings.TrimSpace(res5.Stdout.String()))

			// The behavior is correct: "default" uses the current default level,
			// which was changed to "debug" when we set "all" to "debug"
		})

		t.Run("set all subsystems to 'default' keyword", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			// Get the original default level (just for reference, it should be "error")
			res0 := node.IPFS("log", "level")
			assert.NoError(t, res0.Err)
			// originalDefault := strings.TrimSpace(res0.Stdout.String())
			assert.Equal(t, "error", strings.TrimSpace(res0.Stdout.String()))

			// First set all subsystems to debug
			res1 := node.IPFS("log", "level", "*", "debug")
			assert.NoError(t, res1.Err)
			assert.Contains(t, res1.Stdout.String(), "Changed log level of '*' to 'debug'")

			// Verify a specific subsystem is at debug
			res2 := node.IPFS("log", "level", "core")
			assert.NoError(t, res2.Err)
			assert.Equal(t, "debug", strings.TrimSpace(res2.Stdout.String()))

			// Verify the default level is now debug
			res3 := node.IPFS("log", "level")
			assert.NoError(t, res3.Err)
			assert.Equal(t, "debug", strings.TrimSpace(res3.Stdout.String()))

			// Now set all subsystems back to default (which is now "debug")
			res4 := node.IPFS("log", "level", "*", "default")
			assert.NoError(t, res4.Err)
			assert.Contains(t, res4.Stdout.String(), "Changed log level of '*' to")

			// The subsystem should still be at debug (because that's what default is now)
			res5 := node.IPFS("log", "level", "core")
			assert.NoError(t, res5.Err)
			assert.Equal(t, "debug", strings.TrimSpace(res5.Stdout.String()))

			// The behavior is correct: "default" uses the current default level,
			// which was changed to "debug" when we set "*" to "debug"
		})

		t.Run("shell escaping variants for '*' wildcard", func(t *testing.T) {
			t.Parallel()
			h := harness.NewT(t)
			node := h.NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			// Test different shell escaping methods work for '*'
			// This tests the behavior documented in help text: '*' or "*" or \*

			// Test 1: Single quotes '*' (should work)
			cmd1 := fmt.Sprintf("IPFS_PATH='%s' %s --api='%s' log level '*' info",
				node.Dir, node.IPFSBin, node.APIAddr())
			res1 := h.Sh(cmd1)
			assert.NoError(t, res1.Err)
			assert.Contains(t, res1.Stdout.String(), "Changed log level of '*' to 'info'")

			// Test 2: Double quotes "*" (should work)
			cmd2 := fmt.Sprintf("IPFS_PATH='%s' %s --api='%s' log level \"*\" debug",
				node.Dir, node.IPFSBin, node.APIAddr())
			res2 := h.Sh(cmd2)
			assert.NoError(t, res2.Err)
			assert.Contains(t, res2.Stdout.String(), "Changed log level of '*' to 'debug'")

			// Test 3: Backslash escape \* (should work)
			cmd3 := fmt.Sprintf("IPFS_PATH='%s' %s --api='%s' log level \\* warn",
				node.Dir, node.IPFSBin, node.APIAddr())
			res3 := h.Sh(cmd3)
			assert.NoError(t, res3.Err)
			assert.Contains(t, res3.Stdout.String(), "Changed log level of '*' to 'warn'")

			// Test 4: Verify the final state - should show 'warn' as default
			res4 := node.IPFS("log", "level")
			assert.NoError(t, res4.Err)
			assert.Equal(t, "warn", strings.TrimSpace(res4.Stdout.String()))

			// Test 5: Get all levels using escaped '*' to verify it shows all subsystems
			cmd5 := fmt.Sprintf("IPFS_PATH='%s' %s --api='%s' log level \\*",
				node.Dir, node.IPFSBin, node.APIAddr())
			res5 := h.Sh(cmd5)
			assert.NoError(t, res5.Err)
			output := res5.Stdout.String()
			assert.Contains(t, output, "(default): warn", "Should show updated default level")
			assert.Contains(t, output, "core: warn", "Should show core subsystem at warn level")
		})
	})

	t.Run("HTTP RPC", func(t *testing.T) {
		t.Run("get default level returns JSON", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			// Make HTTP request to get default log level
			resp, err := http.Post(node.APIURL()+"/api/v0/log/level", "", nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Parse JSON response
			var result map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)

			// Check that we have the Levels field
			levels, ok := result["Levels"].(map[string]interface{})
			require.True(t, ok, "Response should have 'Levels' field")

			// Should have exactly one entry for the default level
			assert.Equal(t, 1, len(levels))

			// The default level should be present
			defaultLevel, ok := levels[""]
			require.True(t, ok, "Should have empty string key for default level")
			assert.Equal(t, "error", defaultLevel, "Default level should be 'error'")
		})

		t.Run("get all levels using 'all' returns JSON (alias for '*')", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			expectedSubsystems := getExpectedSubsystems(t, node)

			// Make HTTP request to get all log levels using 'all'
			resp, err := http.Post(node.APIURL()+"/api/v0/log/level?arg=all", "", nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			levels := parseHTTPResponse(t, resp)
			validateAllSubsystemsPresent(t, expectedSubsystems, levels, "JSON response")

			// Should have the (default) entry
			defaultLevel, ok := levels["(default)"]
			require.True(t, ok, "Should have '(default)' key")
			assert.Equal(t, "error", defaultLevel, "Default level should be 'error'")
		})

		t.Run("get all levels returns JSON", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			expectedSubsystems := getExpectedSubsystems(t, node)

			// Make HTTP request to get all log levels
			resp, err := http.Post(node.APIURL()+"/api/v0/log/level?arg=*", "", nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			levels := parseHTTPResponse(t, resp)
			validateAllSubsystemsPresent(t, expectedSubsystems, levels, "JSON response")

			// Should have the (default) entry
			defaultLevel, ok := levels["(default)"]
			require.True(t, ok, "Should have '(default)' key")
			assert.Equal(t, "error", defaultLevel, "Default level should be 'error'")
		})

		t.Run("get specific subsystem level returns JSON", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			// First set a specific level for a subsystem
			resp, err := http.Post(node.APIURL()+"/api/v0/log/level?arg=core&arg=debug", "", nil)
			require.NoError(t, err)
			resp.Body.Close()

			// Now get the level for that subsystem
			resp, err = http.Post(node.APIURL()+"/api/v0/log/level?arg=core", "", nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Parse JSON response
			var result map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)

			// Check that we have the Levels field
			levels, ok := result["Levels"].(map[string]interface{})
			require.True(t, ok, "Response should have 'Levels' field")

			// Should have exactly one entry
			assert.Equal(t, 1, len(levels))

			// Check the level for 'core' subsystem
			coreLevel, ok := levels["core"]
			require.True(t, ok, "Should have 'core' key")
			assert.Equal(t, "debug", coreLevel, "Core level should be 'debug'")
		})

		t.Run("set level using 'all' returns JSON message (alias for '*')", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			// Set a log level using 'all'
			resp, err := http.Post(node.APIURL()+"/api/v0/log/level?arg=all&arg=info", "", nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Parse JSON response
			var result map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)

			// Check that we have the Message field
			message, ok := result["Message"].(string)
			require.True(t, ok, "Response should have 'Message' field")

			// Check the message content (should show '*' in message even when 'all' was used)
			assert.Contains(t, message, "Changed log level of '*' to 'info'")
		})

		t.Run("set level returns JSON message", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			// Set a log level
			resp, err := http.Post(node.APIURL()+"/api/v0/log/level?arg=core&arg=info", "", nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Parse JSON response
			var result map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)

			// Check that we have the Message field
			message, ok := result["Message"].(string)
			require.True(t, ok, "Response should have 'Message' field")

			// Check the message content
			assert.Contains(t, message, "Changed log level of 'core' to 'info'")
		})

		t.Run("set level to 'default' keyword", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			// First set a subsystem to debug
			resp, err := http.Post(node.APIURL()+"/api/v0/log/level?arg=core&arg=debug", "", nil)
			require.NoError(t, err)
			resp.Body.Close()

			// Now set it back to default
			resp, err = http.Post(node.APIURL()+"/api/v0/log/level?arg=core&arg=default", "", nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Parse JSON response
			var result map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)

			// Check that we have the Message field
			message, ok := result["Message"].(string)
			require.True(t, ok, "Response should have 'Message' field")

			// The message should indicate the change
			assert.True(t, strings.Contains(message, "Changed log level of 'core' to"),
				"Message should indicate level change")

			// Verify the level is back to error (default)
			resp, err = http.Post(node.APIURL()+"/api/v0/log/level?arg=core", "", nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			var getResult map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&getResult)
			require.NoError(t, err)

			levels, _ := getResult["Levels"].(map[string]interface{})
			coreLevel, _ := levels["core"].(string)
			assert.Equal(t, "error", coreLevel, "Core level should be back to 'error' (default)")
		})
	})

}

func getExpectedSubsystems(t *testing.T, node *harness.Node) []string {
	t.Helper()
	lsRes := node.IPFS("log", "ls")
	require.NoError(t, lsRes.Err)
	expectedSubsystems := SplitLines(lsRes.Stdout.String())
	assert.Greater(t, len(expectedSubsystems), 10, "Should have many subsystems")
	return expectedSubsystems
}

func parseCLIOutput(t *testing.T, output string) map[string]string {
	t.Helper()
	lines := SplitLines(output)
	actualSubsystems := make(map[string]string)
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, ": ")
		assert.Equal(t, 2, len(parts), "Line should have format 'subsystem: level', got: %s", line)
		assert.NotEmpty(t, parts[0], "Subsystem should not be empty")
		assert.NotEmpty(t, parts[1], "Level should not be empty")
		actualSubsystems[parts[0]] = parts[1]
	}
	return actualSubsystems
}

func parseHTTPResponse(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	levels, ok := result["Levels"].(map[string]interface{})
	require.True(t, ok, "Response should have 'Levels' field")
	assert.Greater(t, len(levels), 10, "Should have many subsystems")
	return levels
}

func validateAllSubsystemsPresent(t *testing.T, expectedSubsystems []string, actualLevels map[string]interface{}, context string) {
	t.Helper()
	for _, expectedSub := range expectedSubsystems {
		expectedSub = strings.TrimSpace(expectedSub)
		if expectedSub == "" {
			continue
		}
		_, found := actualLevels[expectedSub]
		assert.True(t, found, "Expected subsystem '%s' should be present in %s", expectedSub, context)
	}
}

func validateAllSubsystemsPresentCLI(t *testing.T, expectedSubsystems []string, actualLevels map[string]string, context string) {
	t.Helper()
	for _, expectedSub := range expectedSubsystems {
		expectedSub = strings.TrimSpace(expectedSub)
		if expectedSub == "" {
			continue
		}
		_, found := actualLevels[expectedSub]
		assert.True(t, found, "Expected subsystem '%s' should be present in %s", expectedSub, context)
	}
}
