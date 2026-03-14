package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
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

func parseImportedBlockCount(stdout string) int {
	var n int
	for _, line := range testutils.SplitLines(stdout) {
		if _, err := fmt.Sscanf(line, "Imported %d blocks", &n); err == nil {
			return n
		}
	}
	return 0
}

func TestDagExportLocalOnly(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init().StartDaemon()
	defer node.StopDaemon()

	root := node.IPFSAddDeterministic("300KiB", "dag-export-local-only", "--raw-leaves")
	refs := dagRefs(node, root)
	require.GreaterOrEqual(t, len(refs), 2, "need at least root and one child block")

	fullCarPath := filepath.Join(node.Dir, "full.car")
	require.NoError(t, node.IPFSDagExport(root, fullCarPath))
	require.Equal(t, 0, node.RunIPFS("pin", "rm", root).ExitCode())
	require.Equal(t, 0, node.RunIPFS("block", "rm", refs[1]).ExitCode())

	// Export --offline should fail; discard output (no file needed).
	res := node.Runner.Run(harness.RunRequest{
		Path:    node.IPFSBin,
		Args:    []string{"dag", "export", "--offline", root},
		CmdOpts: []harness.CmdOpt{harness.RunWithStdout(io.Discard)},
	})
	require.NotEqual(t, 0, res.ExitCode(), "export --offline without --local-only should fail when a block is missing")
	require.Contains(t, res.Stderr.String(), "block was not found locally")

	partialCarPath := filepath.Join(node.Dir, "partial.car")
	require.NoError(t, node.IPFSDagExport(root, partialCarPath, "--local-only", "--offline"))

	nodeFull := harness.NewT(t).NewNode().Init().StartDaemon()
	defer nodeFull.StopDaemon()
	fullCAR, err := os.Open(fullCarPath)
	require.NoError(t, err)
	defer fullCAR.Close()
	fullRes := nodeFull.Runner.Run(harness.RunRequest{
		Path:    nodeFull.IPFSBin,
		Args:    []string{"dag", "import", "--pin-roots=false", "--stats"},
		CmdOpts: []harness.CmdOpt{harness.RunWithStdin(fullCAR)},
	})
	require.Equal(t, 0, fullRes.ExitCode())
	fullCount := parseImportedBlockCount(fullRes.Stdout.String())
	require.Greater(t, fullCount, 0, "expected 'Imported N blocks' in output: %s", fullRes.Stdout.String())

	nodePartial := harness.NewT(t).NewNode().Init().StartDaemon()
	defer nodePartial.StopDaemon()
	partialCAR, err := os.Open(partialCarPath)
	require.NoError(t, err)
	defer partialCAR.Close()
	partialRes := nodePartial.Runner.Run(harness.RunRequest{
		Path:    nodePartial.IPFSBin,
		Args:    []string{"dag", "import", "--pin-roots=false", "--stats"},
		CmdOpts: []harness.CmdOpt{harness.RunWithStdin(partialCAR)},
	})
	require.Equal(t, 0, partialRes.ExitCode())
	partialCount := parseImportedBlockCount(partialRes.Stdout.String())
	require.Greater(t, partialCount, 0, "expected 'Imported N blocks' in output: %s", partialRes.Stdout.String())

	require.Less(t, partialCount, fullCount, "partial CAR should have fewer blocks than full DAG")
}

func TestDagExportLocalOnlyRequiresOffline(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init().StartDaemon()
	defer node.StopDaemon()

	root := node.IPFSAddDeterministic("300KiB", "dag-local-only-requires-offline", "--raw-leaves")
	refs := dagRefs(node, root)

	require.GreaterOrEqual(t, len(refs), 2)
	require.Equal(t, 0, node.RunIPFS("pin", "rm", root).ExitCode())
	require.Equal(t, 0, node.RunIPFS("block", "rm", refs[1]).ExitCode())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, node.IPFSBin, "dag", "export", "--local-only", root)
	cmd.Env = append(os.Environ(), "IPFS_PATH="+node.Dir)
	cmd.Stdout = io.Discard

	err := cmd.Run()

	require.Error(t, err) // command should fail
}

func TestDagImportPartialCAR(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init().StartDaemon()
	defer node.StopDaemon()

	root := node.IPFSAddDeterministic("300KiB", "dag-import-partial", "--raw-leaves")
	refs := dagRefs(node, root)
	require.GreaterOrEqual(t, len(refs), 2)

	require.Equal(t, 0, node.RunIPFS("pin", "rm", root).ExitCode())
	require.Equal(t, 0, node.RunIPFS("block", "rm", refs[1]).ExitCode())

	partialCarPath := filepath.Join(node.Dir, "partial.car")
	require.NoError(t, node.IPFSDagExport(root, partialCarPath, "--local-only", "--offline"))

	imp := harness.NewT(t).NewNode().Init().StartDaemon()
	defer imp.StopDaemon()
	partialCAR, err := os.Open(partialCarPath)
	require.NoError(t, err)
	defer partialCAR.Close()
	require.NoError(t, imp.IPFSDagImport(partialCAR, root))
}
func TestDagImportLocalOnlyPinRootsConflict(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init().StartDaemon()
	defer node.StopDaemon()

	r, err := os.Open(fixtureFile)
	require.NoError(t, err)
	defer r.Close()

	res := node.Runner.Run(harness.RunRequest{
		Path:    node.IPFSBin,
		Args:    []string{"dag", "import", "--local-only", "--pin-roots"},
		CmdOpts: []harness.CmdOpt{harness.RunWithStdin(r)},
	})

	require.Equal(t, 1, res.ExitCode())
	require.Error(t, res.Err)

	errOutput := res.Stderr.String()

	require.Contains(t, errOutput, "cannot pass both")
	require.Contains(t, errOutput, "pin-roots")
	require.Contains(t, errOutput, "local-only")
}
