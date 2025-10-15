package cli

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/require"
)

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
