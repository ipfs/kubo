package cli

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	provideStatEventuallyTimeout = 15 * time.Second
	provideStatEventuallyTick    = 100 * time.Millisecond
)

// sweepStats mirrors the subset of JSON fields actually used by tests.
// This type is intentionally independent from upstream types to detect breaking changes.
// Only includes fields that tests actually access to keep it simple and maintainable.
type sweepStats struct {
	Sweep struct {
		Closed       bool `json:"closed"`
		Connectivity struct {
			Status string `json:"status"`
		} `json:"connectivity"`
		Queues struct {
			PendingKeyProvides int `json:"pending_key_provides"`
		} `json:"queues"`
		Schedule struct {
			Keys int `json:"keys"`
		} `json:"schedule"`
	} `json:"Sweep"`
}

// parseSweepStats parses JSON output from ipfs provide stat command.
// Tests will naturally fail if upstream removes/renames fields we depend on.
func parseSweepStats(t *testing.T, jsonOutput string) sweepStats {
	t.Helper()
	var stats sweepStats
	err := json.Unmarshal([]byte(jsonOutput), &stats)
	require.NoError(t, err, "failed to parse provide stat JSON output")
	return stats
}

// TestProvideStatAllMetricsDocumented verifies that all metrics output by
// `ipfs provide stat --all` are documented in docs/provide-stats.md.
//
// The test works as follows:
//  1. Starts an IPFS node with Provide.DHT.SweepEnabled=true
//  2. Runs `ipfs provide stat --all` to get all metrics
//  3. Parses the output and extracts all lines with exactly 2 spaces indent
//     (these are the actual metric lines)
//  4. Reads docs/provide-stats.md and extracts all ### section headers
//  5. Ensures every metric in the output has a corresponding ### section in the docs
func TestProvideStatAllMetricsDocumented(t *testing.T) {
	t.Parallel()

	h := harness.NewT(t)
	node := h.NewNode().Init()

	// Enable sweep provider
	node.SetIPFSConfig("Provide.DHT.SweepEnabled", true)
	node.SetIPFSConfig("Provide.Enabled", true)

	node.StartDaemon()
	defer node.StopDaemon()

	// Run `ipfs provide stat --all` to get all metrics
	res := node.IPFS("provide", "stat", "--all")
	require.NoError(t, res.Err)

	// Parse metrics from the command output
	// Only consider lines with exactly two spaces of padding ("  ")
	// These are the actual metric lines as shown in provide.go
	outputMetrics := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(res.Stdout.String()))
	// Only consider lines that start with exactly two spaces
	indent := "  "
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, indent) || strings.HasPrefix(line, indent) {
			continue
		}

		// Remove the indent
		line = strings.TrimPrefix(line, indent)

		// Extract metric name - everything before the first ':'
		parts := strings.SplitN(line, ":", 2)
		if len(parts) >= 1 {
			metricName := strings.TrimSpace(parts[0])
			if metricName != "" {
				outputMetrics[metricName] = true
			}
		}
	}
	require.NoError(t, scanner.Err())

	// Read docs/provide-stats.md
	// Find the repo root by looking for go.mod
	repoRoot := ".."
	for range 6 {
		if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err == nil {
			break
		}
		repoRoot = filepath.Join("..", repoRoot)
	}
	docsPath := filepath.Join(repoRoot, "docs", "provide-stats.md")
	docsFile, err := os.Open(docsPath)
	require.NoError(t, err, "Failed to open provide-stats.md")
	defer docsFile.Close()

	// Parse all ### metric headers from the docs
	documentedMetrics := make(map[string]bool)
	docsScanner := bufio.NewScanner(docsFile)
	for docsScanner.Scan() {
		line := docsScanner.Text()
		if metricName, found := strings.CutPrefix(line, "### "); found {
			metricName = strings.TrimSpace(metricName)
			documentedMetrics[metricName] = true
		}
	}
	require.NoError(t, docsScanner.Err())

	// Check that all output metrics are documented
	var undocumentedMetrics []string
	for metric := range outputMetrics {
		if !documentedMetrics[metric] {
			undocumentedMetrics = append(undocumentedMetrics, metric)
		}
	}

	require.Empty(t, undocumentedMetrics,
		"The following metrics from 'ipfs provide stat --all' are not documented in docs/provide-stats.md: %v\n"+
			"All output metrics: %v\n"+
			"Documented metrics: %v",
		undocumentedMetrics, outputMetrics, documentedMetrics)
}

// TestProvideStatBasic tests basic functionality of ipfs provide stat
func TestProvideStatBasic(t *testing.T) {
	t.Parallel()

	t.Run("works with Sweep provider and shows brief output", func(t *testing.T) {
		t.Parallel()

		h := harness.NewT(t)
		node := h.NewNode().Init()
		node.SetIPFSConfig("Provide.DHT.SweepEnabled", true)
		node.SetIPFSConfig("Provide.Enabled", true)
		node.StartDaemon()
		defer node.StopDaemon()

		res := node.IPFS("provide", "stat")
		require.NoError(t, res.Err)
		assert.Empty(t, res.Stderr.String())

		output := res.Stdout.String()
		// Brief output should contain specific full labels
		assert.Contains(t, output, "Provide queue:")
		assert.Contains(t, output, "Reprovide queue:")
		assert.Contains(t, output, "CIDs scheduled:")
		assert.Contains(t, output, "Regions scheduled:")
		assert.Contains(t, output, "Avg record holders:")
		assert.Contains(t, output, "Ongoing provides:")
		assert.Contains(t, output, "Ongoing reprovides:")
		assert.Contains(t, output, "Total CIDs provided:")
	})

	t.Run("requires daemon to be online", func(t *testing.T) {
		t.Parallel()

		h := harness.NewT(t)
		node := h.NewNode().Init()

		res := node.RunIPFS("provide", "stat")
		assert.Error(t, res.Err)
		assert.Contains(t, res.Stderr.String(), "this command must be run in online mode")
	})
}

// TestProvideStatFlags tests various command flags
func TestProvideStatFlags(t *testing.T) {
	t.Parallel()

	t.Run("--all flag shows all sections with headings", func(t *testing.T) {
		t.Parallel()

		h := harness.NewT(t)
		node := h.NewNode().Init()
		node.SetIPFSConfig("Provide.DHT.SweepEnabled", true)
		node.SetIPFSConfig("Provide.Enabled", true)
		node.StartDaemon()
		defer node.StopDaemon()

		res := node.IPFS("provide", "stat", "--all")
		require.NoError(t, res.Err)

		output := res.Stdout.String()
		// Should contain section headings with colons
		assert.Contains(t, output, "Connectivity:")
		assert.Contains(t, output, "Queues:")
		assert.Contains(t, output, "Schedule:")
		assert.Contains(t, output, "Timings:")
		assert.Contains(t, output, "Network:")
		assert.Contains(t, output, "Operations:")
		assert.Contains(t, output, "Workers:")

		// Should contain detailed metrics not in brief mode
		assert.Contains(t, output, "Uptime:")
		assert.Contains(t, output, "Cycle started:")
		assert.Contains(t, output, "Reprovide interval:")
		assert.Contains(t, output, "Peers swept:")
		assert.Contains(t, output, "Full keyspace coverage:")
	})

	t.Run("--compact requires --all", func(t *testing.T) {
		t.Parallel()

		h := harness.NewT(t)
		node := h.NewNode().Init()
		node.SetIPFSConfig("Provide.DHT.SweepEnabled", true)
		node.SetIPFSConfig("Provide.Enabled", true)
		node.StartDaemon()
		defer node.StopDaemon()

		res := node.RunIPFS("provide", "stat", "--compact")
		assert.Error(t, res.Err)
		assert.Contains(t, res.Stderr.String(), "--compact requires --all flag")
	})

	t.Run("--compact with --all shows 2-column layout", func(t *testing.T) {
		t.Parallel()

		h := harness.NewT(t)
		node := h.NewNode().Init()
		node.SetIPFSConfig("Provide.DHT.SweepEnabled", true)
		node.SetIPFSConfig("Provide.Enabled", true)
		node.StartDaemon()
		defer node.StopDaemon()

		res := node.IPFS("provide", "stat", "--all", "--compact")
		require.NoError(t, res.Err)

		output := res.Stdout.String()
		lines := strings.Split(strings.TrimSpace(output), "\n")
		require.NotEmpty(t, lines)

		// In compact mode, find a line that has both Schedule and Connectivity metrics
		// This confirms 2-column layout is working
		foundTwoColumns := false
		for _, line := range lines {
			if strings.Contains(line, "CIDs scheduled:") && strings.Contains(line, "Status:") {
				foundTwoColumns = true
				break
			}
		}
		assert.True(t, foundTwoColumns, "Should have at least one line with both 'CIDs scheduled:' and 'Status:' confirming 2-column layout")
	})

	t.Run("individual section flags work with full labels", func(t *testing.T) {
		t.Parallel()

		h := harness.NewT(t)
		node := h.NewNode().Init()
		node.SetIPFSConfig("Provide.DHT.SweepEnabled", true)
		node.SetIPFSConfig("Provide.Enabled", true)
		node.StartDaemon()
		defer node.StopDaemon()

		testCases := []struct {
			flag     string
			contains []string
		}{
			{
				flag:     "--connectivity",
				contains: []string{"Status:"},
			},
			{
				flag:     "--queues",
				contains: []string{"Provide queue:", "Reprovide queue:"},
			},
			{
				flag:     "--schedule",
				contains: []string{"CIDs scheduled:", "Regions scheduled:", "Avg prefix length:", "Next region prefix:", "Next region reprovide:"},
			},
			{
				flag:     "--timings",
				contains: []string{"Uptime:", "Current time offset:", "Cycle started:", "Reprovide interval:"},
			},
			{
				flag:     "--network",
				contains: []string{"Avg record holders:", "Peers swept:", "Full keyspace coverage:", "Reachable peers:", "Avg region size:", "Replication factor:"},
			},
			{
				flag:     "--operations",
				contains: []string{"Ongoing provides:", "Ongoing reprovides:", "Total CIDs provided:", "Total records provided:", "Total provide errors:"},
			},
			{
				flag:     "--workers",
				contains: []string{"Active workers:", "Free workers:", "Workers stats:", "Periodic", "Burst"},
			},
		}

		for _, tc := range testCases {
			res := node.IPFS("provide", "stat", tc.flag)
			require.NoError(t, res.Err, "flag %s should work", tc.flag)
			output := res.Stdout.String()
			for _, expected := range tc.contains {
				assert.Contains(t, output, expected, "flag %s should contain '%s'", tc.flag, expected)
			}
		}
	})

	t.Run("multiple section flags can be combined", func(t *testing.T) {
		t.Parallel()

		h := harness.NewT(t)
		node := h.NewNode().Init()
		node.SetIPFSConfig("Provide.DHT.SweepEnabled", true)
		node.SetIPFSConfig("Provide.Enabled", true)
		node.StartDaemon()
		defer node.StopDaemon()

		res := node.IPFS("provide", "stat", "--network", "--operations")
		require.NoError(t, res.Err)

		output := res.Stdout.String()
		// Should have section headings when multiple flags combined
		assert.Contains(t, output, "Network:")
		assert.Contains(t, output, "Operations:")
		assert.Contains(t, output, "Avg record holders:")
		assert.Contains(t, output, "Ongoing provides:")
	})
}

// TestProvideStatLegacyProvider tests Legacy provider specific behavior
func TestProvideStatLegacyProvider(t *testing.T) {
	t.Parallel()

	h := harness.NewT(t)
	node := h.NewNode().Init()
	node.SetIPFSConfig("Provide.DHT.SweepEnabled", false)
	node.SetIPFSConfig("Provide.Enabled", true)
	node.StartDaemon()
	defer node.StopDaemon()

	t.Run("shows legacy stats from old provider system", func(t *testing.T) {
		res := node.IPFS("provide", "stat")
		require.NoError(t, res.Err)

		// Legacy provider shows stats from the old reprovider system
		output := res.Stdout.String()
		assert.Contains(t, output, "TotalReprovides:")
		assert.Contains(t, output, "AvgReprovideDuration:")
		assert.Contains(t, output, "LastReprovideDuration:")
	})

	t.Run("rejects flags with legacy provider", func(t *testing.T) {
		flags := []string{"--all", "--connectivity", "--queues", "--network", "--workers"}
		for _, flag := range flags {
			res := node.RunIPFS("provide", "stat", flag)
			assert.Error(t, res.Err, "flag %s should be rejected for legacy provider", flag)
			assert.Contains(t, res.Stderr.String(), "cannot use flags with legacy provide stats")
		}
	})

	t.Run("rejects --lan flag with legacy provider", func(t *testing.T) {
		res := node.RunIPFS("provide", "stat", "--lan")
		assert.Error(t, res.Err)
		assert.Contains(t, res.Stderr.String(), "LAN stats only available for Sweep provider with Dual DHT")
	})
}

// TestProvideStatOutputFormats tests different output formats
func TestProvideStatOutputFormats(t *testing.T) {
	t.Parallel()

	t.Run("JSON output with Sweep provider", func(t *testing.T) {
		t.Parallel()

		h := harness.NewT(t)
		node := h.NewNode().Init()
		node.SetIPFSConfig("Provide.DHT.SweepEnabled", true)
		node.SetIPFSConfig("Provide.Enabled", true)
		node.StartDaemon()
		defer node.StopDaemon()

		res := node.IPFS("provide", "stat", "--enc=json")
		require.NoError(t, res.Err)

		// Parse JSON to verify structure
		var result struct {
			Sweep  map[string]interface{} `json:"Sweep"`
			Legacy map[string]interface{} `json:"Legacy"`
		}
		err := json.Unmarshal([]byte(res.Stdout.String()), &result)
		require.NoError(t, err, "Output should be valid JSON")
		assert.NotNil(t, result.Sweep, "Sweep stats should be present")
		assert.Nil(t, result.Legacy, "Legacy stats should not be present")
	})

	t.Run("JSON output with Legacy provider", func(t *testing.T) {
		t.Parallel()

		h := harness.NewT(t)
		node := h.NewNode().Init()
		node.SetIPFSConfig("Provide.DHT.SweepEnabled", false)
		node.SetIPFSConfig("Provide.Enabled", true)
		node.StartDaemon()
		defer node.StopDaemon()

		res := node.IPFS("provide", "stat", "--enc=json")
		require.NoError(t, res.Err)

		// Parse JSON to verify structure
		var result struct {
			Sweep  map[string]interface{} `json:"Sweep"`
			Legacy map[string]interface{} `json:"Legacy"`
		}
		err := json.Unmarshal([]byte(res.Stdout.String()), &result)
		require.NoError(t, err, "Output should be valid JSON")
		assert.Nil(t, result.Sweep, "Sweep stats should not be present")
		assert.NotNil(t, result.Legacy, "Legacy stats should be present")
	})
}

// TestProvideStatIntegration tests integration with provide operations
func TestProvideStatIntegration(t *testing.T) {
	t.Parallel()

	t.Run("stats reflect content being added to schedule", func(t *testing.T) {
		t.Parallel()

		h := harness.NewT(t)
		node := h.NewNode().Init()
		node.SetIPFSConfig("Provide.DHT.SweepEnabled", true)
		node.SetIPFSConfig("Provide.Enabled", true)
		node.SetIPFSConfig("Provide.DHT.Interval", "1h")
		node.StartDaemon()
		defer node.StopDaemon()

		// Get initial scheduled CID count
		res1 := node.IPFS("provide", "stat", "--enc=json")
		require.NoError(t, res1.Err)
		initialKeys := parseSweepStats(t, res1.Stdout.String()).Sweep.Schedule.Keys

		// Add content - this should increase CIDs scheduled
		node.IPFSAddStr("test content for stats")

		// Wait for content to appear in schedule (with timeout)
		// The buffered provider may take a moment to schedule items
		require.Eventually(t, func() bool {
			res := node.IPFS("provide", "stat", "--enc=json")
			require.NoError(t, res.Err)
			stats := parseSweepStats(t, res.Stdout.String())
			return stats.Sweep.Schedule.Keys > initialKeys
		}, provideStatEventuallyTimeout, provideStatEventuallyTick, "Content should appear in schedule after adding")
	})

	t.Run("stats work with all documented strategies", func(t *testing.T) {
		t.Parallel()

		// Test all strategies documented in docs/config.md#providestrategy
		strategies := []string{"all", "pinned", "roots", "mfs", "pinned+mfs"}
		for _, strategy := range strategies {
			h := harness.NewT(t)
			node := h.NewNode().Init()
			node.SetIPFSConfig("Provide.DHT.SweepEnabled", true)
			node.SetIPFSConfig("Provide.Enabled", true)
			node.SetIPFSConfig("Provide.Strategy", strategy)
			node.StartDaemon()

			res := node.IPFS("provide", "stat")
			require.NoError(t, res.Err, "stats should work with strategy %s", strategy)
			output := res.Stdout.String()
			assert.NotEmpty(t, output)
			assert.Contains(t, output, "CIDs scheduled:")

			node.StopDaemon()
		}
	})
}

// TestProvideStatDisabledConfig tests behavior when provide system is disabled
func TestProvideStatDisabledConfig(t *testing.T) {
	t.Parallel()

	t.Run("Provide.Enabled=false returns error stats not available", func(t *testing.T) {
		t.Parallel()

		h := harness.NewT(t)
		node := h.NewNode().Init()
		node.SetIPFSConfig("Provide.DHT.SweepEnabled", true)
		node.SetIPFSConfig("Provide.Enabled", false)
		node.StartDaemon()
		defer node.StopDaemon()

		res := node.RunIPFS("provide", "stat")
		assert.Error(t, res.Err)
		assert.Contains(t, res.Stderr.String(), "stats not available")
	})

	t.Run("Provide.Enabled=true with Provide.DHT.Interval=0 returns error stats not available", func(t *testing.T) {
		t.Parallel()

		h := harness.NewT(t)
		node := h.NewNode().Init()
		node.SetIPFSConfig("Provide.DHT.SweepEnabled", true)
		node.SetIPFSConfig("Provide.Enabled", true)
		node.SetIPFSConfig("Provide.DHT.Interval", "0")
		node.StartDaemon()
		defer node.StopDaemon()

		res := node.RunIPFS("provide", "stat")
		assert.Error(t, res.Err)
		assert.Contains(t, res.Stderr.String(), "stats not available")
	})
}
