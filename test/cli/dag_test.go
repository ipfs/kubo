package cli

import (
	"encoding/json"
	"io"
	"os"
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
