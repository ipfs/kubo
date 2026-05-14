package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/require"
)

// TestBalancedDAGLayout verifies that kubo uses the "balanced" DAG layout
// (all leaves at same depth) rather than "balanced-packed" (varying leaf depths).
//
// DAG layout differences across implementations:
//
//   - balanced: kubo, helia (all leaves at same depth, uniform traversal distance)
//   - balanced-packed: singularity (trailing leaves may be at different depths)
//   - trickle: kubo --trickle (varying depths, optimized for append-only/streaming)
//
// kubo does not implement balanced-packed. The trickle layout also produces
// non-uniform leaf depths but with different trade-offs: trickle is optimized
// for append-only and streaming reads (no seeking), while balanced-packed
// minimizes node count.
//
// IPIP-499 documents the balanced vs balanced-packed distinction. Files larger
// than dag_width × chunk_size will have different CIDs between implementations
// using different layouts.
//
// Set DAG_LAYOUT_CAR_OUTPUT environment variable to export CAR files.
// Example: DAG_LAYOUT_CAR_OUTPUT=/tmp/dag-layout go test -run TestBalancedDAGLayout -v
func TestBalancedDAGLayout(t *testing.T) {
	t.Parallel()

	carOutputDir := os.Getenv("DAG_LAYOUT_CAR_OUTPUT")
	exportCARs := carOutputDir != ""
	if exportCARs {
		if err := os.MkdirAll(carOutputDir, 0755); err != nil {
			t.Fatalf("failed to create CAR output directory: %v", err)
		}
		t.Logf("CAR export enabled, writing to: %s", carOutputDir)
	}

	t.Run("balanced layout has uniform leaf depth", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()

		// Create file that triggers multi-level DAG.
		// For default v0: 175 chunks × 256KiB = ~44.8 MiB (just over 174 max links)
		// This creates a 2-level DAG where balanced layout ensures uniform depth.
		fileSize := "45MiB"
		seed := "balanced-test"

		cidStr := node.IPFSAddDeterministic(fileSize, seed)

		// Collect leaf depths by walking DAG
		depths := collectLeafDepths(t, node, cidStr, 0)

		// All leaves must be at same depth for balanced layout
		require.NotEmpty(t, depths, "expected at least one leaf node")
		firstDepth := depths[0]
		for i, d := range depths {
			require.Equal(t, firstDepth, d,
				"leaf %d at depth %d, expected %d (balanced layout requires uniform leaf depth)",
				i, d, firstDepth)
		}
		t.Logf("verified %d leaves all at depth %d (CID: %s)", len(depths), firstDepth, cidStr)

		if exportCARs {
			carPath := filepath.Join(carOutputDir, "balanced_"+fileSize+".car")
			require.NoError(t, node.IPFSDagExport(cidStr, carPath))
			t.Logf("exported: %s -> %s", cidStr, carPath)
		}
	})

	t.Run("trickle layout has varying leaf depth", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()

		fileSize := "45MiB"
		seed := "trickle-test"

		// Add with trickle layout (--trickle flag).
		// Trickle produces non-uniform leaf depths, optimized for append-only
		// and streaming reads (no seeking). This subtest validates the test
		// logic by confirming we can detect varying depths.
		cidStr := node.IPFSAddDeterministic(fileSize, seed, "--trickle")

		depths := collectLeafDepths(t, node, cidStr, 0)

		// Trickle layout should have varying depths
		require.NotEmpty(t, depths, "expected at least one leaf node")
		minDepth, maxDepth := depths[0], depths[0]
		for _, d := range depths {
			if d < minDepth {
				minDepth = d
			}
			if d > maxDepth {
				maxDepth = d
			}
		}
		require.NotEqual(t, minDepth, maxDepth,
			"trickle layout should have varying leaf depths, got uniform depth %d", minDepth)
		t.Logf("verified %d leaves with depths ranging from %d to %d (CID: %s)", len(depths), minDepth, maxDepth, cidStr)

		if exportCARs {
			carPath := filepath.Join(carOutputDir, "trickle_"+fileSize+".car")
			require.NoError(t, node.IPFSDagExport(cidStr, carPath))
			t.Logf("exported: %s -> %s", cidStr, carPath)
		}
	})
}

// collectLeafDepths recursively walks DAG and returns depth of each leaf node.
// A node is a leaf if it's a raw block or a dag-pb node with no links.
func collectLeafDepths(t *testing.T, node *harness.Node, cid string, depth int) []int {
	t.Helper()

	// Check codec to see if this is a raw leaf
	res := node.IPFS("cid", "format", "-f", "%c", cid)
	codec := strings.TrimSpace(res.Stdout.String())
	if codec == "raw" {
		// Raw blocks are always leaves
		return []int{depth}
	}

	// Try to inspect as dag-pb node
	pbNode, err := node.InspectPBNode(cid)
	if err != nil {
		// Can't parse as dag-pb, treat as leaf
		return []int{depth}
	}

	// No links = leaf node
	if len(pbNode.Links) == 0 {
		return []int{depth}
	}

	// Recurse into children
	var depths []int
	for _, link := range pbNode.Links {
		childDepths := collectLeafDepths(t, node, link.Hash.Slash, depth+1)
		depths = append(depths, childDepths...)
	}
	return depths
}
