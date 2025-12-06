package rpc

import (
	"context"
	"os"
	"testing"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/core/coreiface/options"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/require"
)

func TestDagImport_Basic(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	h := harness.NewT(t)
	node := h.NewNode().Init().StartDaemon()
	defer node.StopDaemon()

	apiMaddr, err := node.TryAPIAddr()
	require.NoError(t, err)

	api, err := NewApi(apiMaddr)
	require.NoError(t, err)

	// Open test fixture
	carFile, err := os.Open("../../test/cli/fixtures/TestDagStat.car")
	require.NoError(t, err)
	defer carFile.Close()

	// Import CAR file
	results, err := api.Dag().Import(ctx, files.NewReaderFile(carFile))
	require.NoError(t, err)

	// Collect results
	var roots []cid.Cid
	for result := range results {
		if result.Root != nil {
			roots = append(roots, result.Root.Cid)
			require.Empty(t, result.Root.PinErrorMsg, "pin should succeed")
		}
	}

	// Verify we got exactly one root
	require.Len(t, roots, 1, "should have exactly one root")

	// Verify the expected root CID
	expectedRoot := "bafyreifrm6uf5o4dsaacuszf35zhibyojlqclabzrms7iak67pf62jygaq"
	require.Equal(t, expectedRoot, roots[0].String())
}

func TestDagImport_WithStats(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	h := harness.NewT(t)
	node := h.NewNode().Init().StartDaemon()
	defer node.StopDaemon()

	apiMaddr, err := node.TryAPIAddr()
	require.NoError(t, err)

	api, err := NewApi(apiMaddr)
	require.NoError(t, err)

	carFile, err := os.Open("../../test/cli/fixtures/TestDagStat.car")
	require.NoError(t, err)
	defer carFile.Close()

	// Import with stats enabled
	results, err := api.Dag().Import(ctx, files.NewReaderFile(carFile),
		options.Dag.Stats(true))
	require.NoError(t, err)

	var roots []cid.Cid
	var gotStats bool
	var blockCount uint64

	for result := range results {
		if result.Root != nil {
			roots = append(roots, result.Root.Cid)
		}
		if result.Stats != nil {
			gotStats = true
			blockCount = result.Stats.BlockCount
		}
	}

	require.Len(t, roots, 1, "should have one root")
	require.True(t, gotStats, "should receive stats")
	require.Equal(t, uint64(4), blockCount, "TestDagStat.car has 4 blocks")
}

func TestDagImport_OfflineWithFastProvide(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	h := harness.NewT(t)
	node := h.NewNode().Init().StartDaemon("--offline=true")
	defer node.StopDaemon()

	apiMaddr, err := node.TryAPIAddr()
	require.NoError(t, err)

	api, err := NewApi(apiMaddr)
	require.NoError(t, err)

	carFile, err := os.Open("../../test/cli/fixtures/TestDagStat.car")
	require.NoError(t, err)
	defer carFile.Close()

	// Import with fast-provide enabled in offline mode
	// Should succeed gracefully (fast-provide silently skipped)
	results, err := api.Dag().Import(ctx, files.NewReaderFile(carFile),
		options.Dag.FastProvideRoot(true),
		options.Dag.FastProvideWait(true))
	require.NoError(t, err)

	var roots []cid.Cid
	for result := range results {
		if result.Root != nil {
			roots = append(roots, result.Root.Cid)
		}
	}

	require.Len(t, roots, 1, "import should succeed offline with fast-provide enabled")
}

func TestDagImport_OnlineWithFastProvideWait(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	h := harness.NewT(t)
	node := h.NewNode().Init().StartDaemon()
	defer node.StopDaemon()

	apiMaddr, err := node.TryAPIAddr()
	require.NoError(t, err)

	api, err := NewApi(apiMaddr)
	require.NoError(t, err)

	carFile, err := os.Open("../../test/cli/fixtures/TestDagStat.car")
	require.NoError(t, err)
	defer carFile.Close()

	// Import with fast-provide wait enabled in online mode.
	// This tests that FastProvideWait actually blocks (not fire-and-forget).
	// In isolated test environment with no DHT peers, the blocking provide
	// operation should fail and propagate an error.
	results, err := api.Dag().Import(ctx, files.NewReaderFile(carFile),
		options.Dag.FastProvideRoot(true),
		options.Dag.FastProvideWait(true))

	// Initial call may succeed, but we should get error from results channel
	if err == nil {
		// Consume results until we hit the expected error
		var gotError bool
		for result := range results {
			if result.Err != nil {
				gotError = true
				require.Contains(t, result.Err.Error(), "fast-provide",
					"error should be from fast-provide operation")
				break
			}
		}
		require.True(t, gotError, "should receive fast-provide error in isolated test environment")
	} else {
		// Error returned directly (also acceptable)
		require.Contains(t, err.Error(), "fast-provide",
			"error should be from fast-provide operation")
	}
}
