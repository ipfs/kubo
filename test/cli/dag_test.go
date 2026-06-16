package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/ipfs/kubo/test/cli/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fixtureFile    = "./fixtures/TestDagStat.car"
	textOutputPath = "./fixtures/TestDagStatExpectedOutput.txt"
	node1Cid       = "bafyreibmdfd7c5db4kls4ty57zljfhqv36gi43l6txl44pi423wwmeskwy"
	node2Cid       = "bafyreie3njilzdi4ixumru4nzgecsnjtu7fzfcwhg7e6s4s5i7cnbslvn4"
	fixtureCid     = "bafyreifrm6uf5o4dsaacuszf35zhibyojlqclabzrms7iak67pf62jygaq"
)

type DagStat struct {
	Cid       string `json:"Cid"`
	Size      int    `json:"Size"`
	NumBlocks int    `json:"NumBlocks"`
}

type Data struct {
	UniqueBlocks int       `json:"UniqueBlocks"`
	TotalSize    int       `json:"TotalSize"`
	SharedSize   int       `json:"SharedSize"`
	Ratio        float64   `json:"Ratio"`
	DagStats     []DagStat `json:"DagStats"`
}

// The Fixture file represents a dag where 2 nodes of size = 46B each, have a common child of 7B
// when traversing the DAG from the root's children (node1 and node2) we count (46 + 7)x2 bytes (counting redundant bytes) = 106
// since both nodes share a common child of 7 bytes we actually had to read (46)x2 + 7 =  99 bytes
// we should get a dedup ratio of 106/99 that results in approximately 1.0707071

func TestDag(t *testing.T) {
	t.Parallel()

	t.Run("ipfs dag stat --enc=json", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		// Import fixture
		r, err := os.Open(fixtureFile)
		assert.Nil(t, err)
		defer r.Close()
		err = node.IPFSDagImport(r, fixtureCid)
		assert.NoError(t, err)
		stat := node.RunIPFS("dag", "stat", "--progress=false", "--enc=json", node1Cid, node2Cid)
		var data Data
		err = json.Unmarshal(stat.Stdout.Bytes(), &data)
		assert.NoError(t, err)

		expectedUniqueBlocks := 3
		expectedSharedSize := 7
		expectedTotalSize := 99
		expectedRatio := float64(expectedSharedSize+expectedTotalSize) / float64(expectedTotalSize)
		expectedDagStatsLength := 2
		// Validate UniqueBlocks
		assert.Equal(t, expectedUniqueBlocks, data.UniqueBlocks)
		assert.Equal(t, expectedSharedSize, data.SharedSize)
		assert.Equal(t, expectedTotalSize, data.TotalSize)
		assert.Equal(t, testutils.FloatTruncate(expectedRatio, 4), testutils.FloatTruncate(data.Ratio, 4))

		// Validate DagStats
		assert.Equal(t, expectedDagStatsLength, len(data.DagStats))
		node1Output := data.DagStats[0]
		node2Output := data.DagStats[1]

		assert.Equal(t, node1Output.Cid, node1Cid)
		assert.Equal(t, node2Output.Cid, node2Cid)

		expectedNode1Size := (expectedTotalSize + expectedSharedSize) / 2
		expectedNode2Size := (expectedTotalSize + expectedSharedSize) / 2
		assert.Equal(t, expectedNode1Size, node1Output.Size)
		assert.Equal(t, expectedNode2Size, node2Output.Size)

		expectedNode1Blocks := 2
		expectedNode2Blocks := 2
		assert.Equal(t, expectedNode1Blocks, node1Output.NumBlocks)
		assert.Equal(t, expectedNode2Blocks, node2Output.NumBlocks)
	})

	t.Run("ipfs dag stat", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()
		r, err := os.Open(fixtureFile)
		assert.NoError(t, err)
		defer r.Close()
		f, err := os.Open(textOutputPath)
		assert.NoError(t, err)
		defer f.Close()
		content, err := io.ReadAll(f)
		assert.NoError(t, err)
		err = node.IPFSDagImport(r, fixtureCid)
		assert.NoError(t, err)
		stat := node.RunIPFS("dag", "stat", "--progress=false", node1Cid, node2Cid)
		assert.Equal(t, content, stat.Stdout.Bytes())
	})

	t.Run("ipfs dag stat single root", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		r, err := os.Open(fixtureFile)
		assert.NoError(t, err)
		defer r.Close()
		err = node.IPFSDagImport(r, fixtureCid)
		assert.NoError(t, err)

		// Stat a single root. boxo dedups shared blocks during the traversal, so
		// the result reports no redundancy: SharedSize is 0 and Ratio is 1.
		stat := node.RunIPFS("dag", "stat", "--progress=false", "--enc=json", fixtureCid)
		var data Data
		err = json.Unmarshal(stat.Stdout.Bytes(), &data)
		assert.NoError(t, err)

		// root (95B) + node1 (46B) + node2 (46B) + shared child (7B) = 4 blocks, 194B
		assert.Equal(t, 4, data.UniqueBlocks)
		assert.Equal(t, 194, data.TotalSize)
		assert.Equal(t, 0, data.SharedSize)
		assert.Equal(t, float64(1), data.Ratio)

		// With one root, every counted block is unique, so the summary totals
		// match that root's own block count and size.
		require.Len(t, data.DagStats, 1)
		assert.Equal(t, fixtureCid, data.DagStats[0].Cid)
		assert.Equal(t, data.UniqueBlocks, data.DagStats[0].NumBlocks)
		assert.Equal(t, data.TotalSize, data.DagStats[0].Size)
	})
}

func TestDagImportCARv2(t *testing.T) {
	t.Parallel()
	// Regression test for https://github.com/ipfs/kubo/issues/9361
	// CARv2 import fails with "operation not supported" when using the HTTP API
	// because the multipart reader doesn't support seeking, but the boxo
	// ReaderFile falsely advertises io.Seeker compliance.

	carv2Fixture := "./fixtures/TestDagStatCARv2.car"

	t.Run("CARv2 import via HTTP API (online)", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		r, err := os.Open(carv2Fixture)
		require.NoError(t, err)
		defer r.Close()

		// Use Runner.Run (not MustRun) so the test captures errors
		// instead of panicking -- this lets us assert on the result.
		res := node.Runner.Run(harness.RunRequest{
			Path: node.IPFSBin,
			Args: []string{"dag", "import", "--pin-roots=false"},
			CmdOpts: []harness.CmdOpt{
				harness.RunWithStdin(r),
			},
		})
		require.Equal(t, 0, res.ExitCode(), "CARv2 import should succeed over HTTP API, stderr: %s", res.Stderr.String())

		// Verify the imported blocks are accessible
		stat := node.RunIPFS("dag", "stat", "--progress=false", "--enc=json", fixtureCid)
		var data Data
		err = json.Unmarshal(stat.Stdout.Bytes(), &data)
		require.NoError(t, err)
		// root + node1 + node2 + shared child = 4 unique blocks
		require.Equal(t, 4, data.UniqueBlocks)
	})
}

func TestDagImportFastProvide(t *testing.T) {
	t.Parallel()

	t.Run("fast-provide-root disabled via config: verify skipped in logs", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.FastProvideRoot = config.False
		})

		// Start daemon with debug logging
		node.StartDaemonWithReq(harness.RunRequest{
			CmdOpts: []harness.CmdOpt{
				harness.RunWithEnv(map[string]string{
					"GOLOG_LOG_LEVEL": "error,core/commands=debug,core/commands/cmdenv=debug",
				}),
			},
		}, "")
		defer node.StopDaemon()

		// Import CAR file
		r, err := os.Open(fixtureFile)
		require.NoError(t, err)
		defer r.Close()
		err = node.IPFSDagImport(r, fixtureCid)
		require.NoError(t, err)

		// Verify fast-provide-root was disabled
		daemonLog := node.Daemon.Stderr.String()
		require.Contains(t, daemonLog, "fast-provide-root: skipped")
	})

	t.Run("fast-provide-root enabled with wait=false: verify async provide", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		// Use default config (FastProvideRoot=true, FastProvideWait=false)

		node.StartDaemonWithReq(harness.RunRequest{
			CmdOpts: []harness.CmdOpt{
				harness.RunWithEnv(map[string]string{
					"GOLOG_LOG_LEVEL": "error,core/commands=debug,core/commands/cmdenv=debug",
				}),
			},
		}, "")
		defer node.StopDaemon()

		// Import CAR file
		r, err := os.Open(fixtureFile)
		require.NoError(t, err)
		defer r.Close()
		err = node.IPFSDagImport(r, fixtureCid)
		require.NoError(t, err)

		daemonLog := node.Daemon.Stderr
		// Should see async mode started
		require.Contains(t, daemonLog.String(), "fast-provide-root: enabled")
		require.Contains(t, daemonLog.String(), "fast-provide-root: providing asynchronously")
		require.Contains(t, daemonLog.String(), fixtureCid) // Should log the specific CID being provided

		// Wait for async completion or failure (slightly more than DefaultFastProvideTimeout)
		// In test environment with no DHT peers, this will fail with "failed to find any peer in table"
		timeout := config.DefaultFastProvideTimeout + time.Second
		completedOrFailed := waitForLogMessage(daemonLog, "async provide completed", timeout) ||
			waitForLogMessage(daemonLog, "async provide failed", timeout)
		require.True(t, completedOrFailed, "async provide should complete or fail within timeout")
	})

	t.Run("fast-provide-root enabled with wait=true: verify sync provide", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.FastProvideWait = config.True
		})

		node.StartDaemonWithReq(harness.RunRequest{
			CmdOpts: []harness.CmdOpt{
				harness.RunWithEnv(map[string]string{
					"GOLOG_LOG_LEVEL": "error,core/commands=debug,core/commands/cmdenv=debug",
				}),
			},
		}, "")
		defer node.StopDaemon()

		// Import CAR file - use Run instead of IPFSDagImport to handle expected error
		r, err := os.Open(fixtureFile)
		require.NoError(t, err)
		defer r.Close()
		res := node.Runner.Run(harness.RunRequest{
			Path: node.IPFSBin,
			Args: []string{"dag", "import", "--pin-roots=false"},
			CmdOpts: []harness.CmdOpt{
				harness.RunWithStdin(r),
			},
		})
		// In sync mode (wait=true), provide errors propagate and fail the command.
		// Test environment uses 'test' profile with no bootstrappers, and CI has
		// insufficient peers for proper DHT puts, so we expect this to fail with
		// "failed to find any peer in table" error from the DHT.
		require.Equal(t, 1, res.ExitCode())
		require.Contains(t, res.Stderr.String(), "Error: fast-provide: failed to find any peer in table")

		daemonLog := node.Daemon.Stderr.String()
		// Should see sync mode started
		require.Contains(t, daemonLog, "fast-provide-root: enabled")
		require.Contains(t, daemonLog, "fast-provide-root: providing synchronously")
		require.Contains(t, daemonLog, fixtureCid)            // Should log the specific CID being provided
		require.Contains(t, daemonLog, "sync provide failed") // Verify the failure was logged
	})

	t.Run("fast-provide-wait ignored when root disabled", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.FastProvideRoot = config.False
			cfg.Import.FastProvideWait = config.True
		})

		node.StartDaemonWithReq(harness.RunRequest{
			CmdOpts: []harness.CmdOpt{
				harness.RunWithEnv(map[string]string{
					"GOLOG_LOG_LEVEL": "error,core/commands=debug,core/commands/cmdenv=debug",
				}),
			},
		}, "")
		defer node.StopDaemon()

		// Import CAR file
		r, err := os.Open(fixtureFile)
		require.NoError(t, err)
		defer r.Close()
		err = node.IPFSDagImport(r, fixtureCid)
		require.NoError(t, err)

		daemonLog := node.Daemon.Stderr.String()
		require.Contains(t, daemonLog, "fast-provide-root: skipped")
		// Note: dag import doesn't log wait-flag-ignored like add does
	})

	t.Run("CLI flag overrides config: flag=true overrides config=false", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.FastProvideRoot = config.False
		})

		node.StartDaemonWithReq(harness.RunRequest{
			CmdOpts: []harness.CmdOpt{
				harness.RunWithEnv(map[string]string{
					"GOLOG_LOG_LEVEL": "error,core/commands=debug,core/commands/cmdenv=debug",
				}),
			},
		}, "")
		defer node.StopDaemon()

		// Import CAR file with flag override
		r, err := os.Open(fixtureFile)
		require.NoError(t, err)
		defer r.Close()
		err = node.IPFSDagImport(r, fixtureCid, "--fast-provide-root=true")
		require.NoError(t, err)

		daemonLog := node.Daemon.Stderr
		// Flag should enable it despite config saying false
		require.Contains(t, daemonLog.String(), "fast-provide-root: enabled")
		require.Contains(t, daemonLog.String(), "fast-provide-root: providing asynchronously")
		require.Contains(t, daemonLog.String(), fixtureCid) // Should log the specific CID being provided
	})

	t.Run("CLI flag overrides config: flag=false overrides config=true", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.FastProvideRoot = config.True
		})

		node.StartDaemonWithReq(harness.RunRequest{
			CmdOpts: []harness.CmdOpt{
				harness.RunWithEnv(map[string]string{
					"GOLOG_LOG_LEVEL": "error,core/commands=debug,core/commands/cmdenv=debug",
				}),
			},
		}, "")
		defer node.StopDaemon()

		// Import CAR file with flag override
		r, err := os.Open(fixtureFile)
		require.NoError(t, err)
		defer r.Close()
		err = node.IPFSDagImport(r, fixtureCid, "--fast-provide-root=false")
		require.NoError(t, err)

		daemonLog := node.Daemon.Stderr.String()
		// Flag should disable it despite config saying true
		require.Contains(t, daemonLog, "fast-provide-root: skipped")
	})
}

// dagRefs returns root plus recursive ref CIDs from "ipfs refs -r --unique root".
func dagRefs(node *harness.Node, root string) []string {
	refsRes := node.IPFS("refs", "-r", "--unique", root)
	refs := []string{root}
	for _, line := range testutils.SplitLines(strings.TrimSpace(refsRes.Stdout.String())) {
		if line != "" {
			refs = append(refs, line)
		}
	}
	return refs
}

// countCARBlocks imports the CAR at carPath onto a fresh node and returns the
// number of blocks reported by `dag import --stats`. The fresh node guarantees
// the count reflects what is in the CAR, not what was already in the store.
func countCARBlocks(t *testing.T, carPath string) int {
	t.Helper()
	node := harness.NewT(t).NewNode().Init().StartDaemon()
	defer node.StopDaemon()

	car, err := os.Open(carPath)
	require.NoError(t, err)
	defer car.Close()

	res := node.Runner.Run(harness.RunRequest{
		Path:    node.IPFSBin,
		Args:    []string{"dag", "import", "--pin-roots=false", "--stats"},
		CmdOpts: []harness.CmdOpt{harness.RunWithStdin(car)},
	})
	require.Equal(t, 0, res.ExitCode(), "dag import --stats failed: %s", res.Stderr.String())

	var n int
	for _, line := range testutils.SplitLines(res.Stdout.String()) {
		if _, err := fmt.Sscanf(line, "Imported %d blocks", &n); err == nil {
			break
		}
	}
	require.Greater(t, n, 0, "expected 'Imported N blocks' in stdout: %q", res.Stdout.String())
	return n
}

// shallowDAGArgs are the `ipfs add` args used by the partial-DAG helpers
// below. Chunker and max-file-links are pinned so the resulting DAG shape
// (root + 2 raw leaves) is independent of changes to Import.* defaults or
// applied profiles.
var shallowDAGArgs = []string{"--raw-leaves", "--chunker=size-262144", "--max-file-links=174"}

// makePartialDAG adds a 300 KiB file with shallowDAGArgs (yielding root + 2
// raw leaves) and then deletes the first leaf so the node holds a DAG with
// one missing block. Returns the root CID and the CID that was removed.
func makePartialDAG(t *testing.T, node *harness.Node, seed string, addArgs ...string) (root, removed string) {
	t.Helper()
	root = node.IPFSAddDeterministic("300KiB", seed, append(shallowDAGArgs, addArgs...)...)
	refs := dagRefs(node, root)
	require.Equal(t, 3, len(refs), "expected exactly root + 2 raw leaves with pinned chunker/max-links, got %v", refs)
	require.Equal(t, 0, node.RunIPFS("pin", "rm", root).ExitCode())
	require.Equal(t, 0, node.RunIPFS("block", "rm", refs[1]).ExitCode())
	return root, refs[1]
}

// TestDagExportLocalOnly verifies the core promise of --local-only: a DAG
// with a single missing leaf can still be exported as a partial CAR, and
// the partial CAR contains exactly the full DAG minus the removed block.
func TestDagExportLocalOnly(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init().StartDaemon()
	defer node.StopDaemon()

	// Snapshot the full DAG to a CAR before the block is removed, so we
	// have a baseline block count to compare against.
	root := node.IPFSAddDeterministic("300KiB", "dag-export-local-only", shallowDAGArgs...)
	fullCarPath := filepath.Join(node.Dir, "full.car")
	require.NoError(t, node.IPFSDagExport(root, fullCarPath))
	fullCount := countCARBlocks(t, fullCarPath)
	require.Equal(t, 3, fullCount, "expected root + 2 raw leaves (full=%d)", fullCount)

	// Drop one leaf so the local DAG is partial.
	refs := dagRefs(node, root)
	require.Equal(t, 0, node.RunIPFS("pin", "rm", root).ExitCode())
	require.Equal(t, 0, node.RunIPFS("block", "rm", refs[1]).ExitCode())

	// Sanity: plain --offline (without --local-only) must fail loudly
	// when a block is missing. This guards the existing behavior.
	res := node.Runner.Run(harness.RunRequest{
		Path:    node.IPFSBin,
		Args:    []string{"dag", "export", "--offline", root},
		CmdOpts: []harness.CmdOpt{harness.RunWithStdout(io.Discard)},
	})
	require.NotEqual(t, 0, res.ExitCode(), "dag export --offline must fail when a block is missing")
	require.Contains(t, res.Stderr.String(), "block was not found locally")

	// --local-only must succeed and produce a CAR with exactly the
	// full DAG minus the one removed leaf.
	partialCarPath := filepath.Join(node.Dir, "partial.car")
	require.NoError(t, node.IPFSDagExport(root, partialCarPath, "--local-only", "--offline"))
	partialCount := countCARBlocks(t, partialCarPath)

	require.Equal(t, fullCount-1, partialCount,
		"partial CAR should be exactly the full DAG minus the one removed leaf (full=%d, partial=%d)",
		fullCount, partialCount)
}

// TestDagExportLocalOnlyImpliesOffline verifies that --local-only on its own
// makes a partial-DAG export succeed: it implies --offline so the user does
// not have to pass both flags.
func TestDagExportLocalOnlyImpliesOffline(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init().StartDaemon()
	defer node.StopDaemon()

	root, _ := makePartialDAG(t, node, "dag-export-local-only-implies")

	// Export with only --local-only (no --offline) and confirm the
	// resulting CAR has the right number of blocks (full DAG minus one).
	partialCarPath := filepath.Join(node.Dir, "partial.car")
	require.NoError(t, node.IPFSDagExport(root, partialCarPath, "--local-only"))

	// 300KiB --raw-leaves yields root + 2 leaves, so removing one leaf
	// leaves 2 blocks. Asserting the exact count proves --offline was
	// actually applied (without it, the export would either fetch the
	// missing block or fail differently).
	require.Equal(t, 2, countCARBlocks(t, partialCarPath))
}

// TestDagExportLocalOnlySkipsSubtree verifies that when a non-leaf block is
// missing, --local-only skips the entire subtree under it, not just the
// missing block. Uses a small chunk size to force a depth>1 DAG so removing
// an intermediate prunes many descendant blocks.
func TestDagExportLocalOnlySkipsSubtree(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init().StartDaemon()
	defer node.StopDaemon()

	// chunker=size-256 + 64 KiB → 256 leaves; max-file-links=174 forces
	// at least one intermediate dag-pb layer between root and leaves
	// (256 > 174). Both values are pinned so the DAG shape (and the
	// counts below) survives any change to Import.* defaults or profiles.
	root := node.IPFSAddDeterministic("64KiB", "dag-export-local-only-subtree",
		"--raw-leaves", "--chunker=size-256", "--max-file-links=174")
	fullCarPath := filepath.Join(node.Dir, "full.car")
	require.NoError(t, node.IPFSDagExport(root, fullCarPath))
	fullCount := countCARBlocks(t, fullCarPath)
	// 1 root + 2 intermediates (174 + 82 children) + 256 leaves = 259.
	require.Equal(t, 259, fullCount, "expected root + 2 intermediates + 256 leaves, got %d", fullCount)

	// Find the first intermediate ref: a non-leaf whose codec is dag-pb.
	// "ipfs refs -r --unique" lists CIDs depth-first; the root's first
	// child in a balanced UnixFS DAG with >174 leaves is an intermediate.
	refs := dagRefs(node, root)
	intermediate := refs[1]
	intermediateChildren := dagRefs(node, intermediate)
	require.Greater(t, len(intermediateChildren), 10,
		"expected refs[1] to be a non-leaf with many children, got %d", len(intermediateChildren))

	// Remove the intermediate. Its subtree blocks remain locally, but
	// without the intermediate the walker cannot reach them, so they
	// must be skipped along with it.
	require.Equal(t, 0, node.RunIPFS("pin", "rm", root).ExitCode())
	require.Equal(t, 0, node.RunIPFS("block", "rm", intermediate).ExitCode())

	partialCarPath := filepath.Join(node.Dir, "partial.car")
	require.NoError(t, node.IPFSDagExport(root, partialCarPath, "--local-only"))
	partialCount := countCARBlocks(t, partialCarPath)

	expectedDropped := len(intermediateChildren) // includes the intermediate itself
	require.Equal(t, fullCount-expectedDropped, partialCount,
		"removing intermediate %s should drop it and its %d descendants (full=%d, partial=%d)",
		intermediate, expectedDropped-1, fullCount, partialCount)
}

// TestDagExportLocalOnlyConflictsWithOnline verifies that explicitly asking
// for online mode together with --local-only is rejected, since the two
// settings contradict each other.
func TestDagExportLocalOnlyConflictsWithOnline(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init().StartDaemon()
	defer node.StopDaemon()

	root := node.IPFSAddDeterministic("300KiB", "dag-export-local-only-online", "--raw-leaves")

	res := node.RunIPFS("dag", "export", "--local-only", "--offline=false", root)
	require.NotEqual(t, 0, res.ExitCode(), "dag export --local-only --offline=false should be rejected")
	stderr := res.Stderr.String()
	require.Contains(t, stderr, "--local-only")
	require.Contains(t, stderr, "--offline")
}

// TestDagImportPartialCAR is the round-trip happy path: a partial CAR from
// --local-only can be imported on a fresh node with default flags (the
// IPFSDagImport harness helper passes --pin-roots=false). The helper also
// confirms the root resolves offline on the receiver.
func TestDagImportPartialCAR(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init().StartDaemon()
	defer node.StopDaemon()

	root, _ := makePartialDAG(t, node, "dag-import-partial")

	partialCarPath := filepath.Join(node.Dir, "partial.car")
	require.NoError(t, node.IPFSDagExport(root, partialCarPath, "--local-only", "--offline"))

	imp := harness.NewT(t).NewNode().Init().StartDaemon()
	defer imp.StopDaemon()
	partialCAR, err := os.Open(partialCarPath)
	require.NoError(t, err)
	defer partialCAR.Close()
	require.NoError(t, imp.IPFSDagImport(partialCAR, root))
}

// TestDagImportLocalOnlyImpliesNoPin verifies that --local-only on its own
// makes a partial-CAR import succeed: it implies --pin-roots=false so the
// user does not have to pass both flags.
func TestDagImportLocalOnlyImpliesNoPin(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init().StartDaemon()
	defer node.StopDaemon()

	root, _ := makePartialDAG(t, node, "dag-import-local-only-implies")
	partialCarPath := filepath.Join(node.Dir, "partial.car")
	require.NoError(t, node.IPFSDagExport(root, partialCarPath, "--local-only", "--offline"))

	imp := harness.NewT(t).NewNode().Init().StartDaemon()
	defer imp.StopDaemon()
	partialCAR, err := os.Open(partialCarPath)
	require.NoError(t, err)
	defer partialCAR.Close()

	// Import with only --local-only (no --pin-roots=false). Should
	// succeed because --local-only implies --pin-roots=false, and the
	// receiver must not attempt to pin (pin would fail on a partial DAG).
	res := imp.Runner.Run(harness.RunRequest{
		Path:    imp.IPFSBin,
		Args:    []string{"dag", "import", "--local-only"},
		CmdOpts: []harness.CmdOpt{harness.RunWithStdin(partialCAR)},
	})
	require.Equal(t, 0, res.ExitCode(),
		"dag import --local-only on a partial CAR should succeed; stderr: %s", res.Stderr.String())
	require.NotContains(t, res.Stdout.String(), "Pinned root",
		"import must not pin when --local-only is set")
}

// TestDagImportLocalOnlyPinRootsConflict verifies that --local-only is
// rejected when combined with an explicit --pin-roots=true. The two are
// mutually exclusive: --local-only is for partial CARs (no full DAG to pin).
func TestDagImportLocalOnlyPinRootsConflict(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init().StartDaemon()
	defer node.StopDaemon()

	r, err := os.Open(fixtureFile)
	require.NoError(t, err)
	defer r.Close()

	res := node.Runner.Run(harness.RunRequest{
		Path:    node.IPFSBin,
		Args:    []string{"dag", "import", "--local-only", "--pin-roots=true"},
		CmdOpts: []harness.CmdOpt{harness.RunWithStdin(r)},
	})

	require.NotEqual(t, 0, res.ExitCode())
	stderr := res.Stderr.String()
	require.Contains(t, stderr, "--local-only")
	require.Contains(t, stderr, "--pin-roots")
}
