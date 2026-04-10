package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/require"
)

// TestCidBase verifies that --cid-base is respected across commands
// and that CIDv0 is auto-upgraded to CIDv1 when a non-base58btc base
// is requested.
func TestCidBase(t *testing.T) {
	t.Parallel()

	makeDaemon := func(t *testing.T) *harness.Node {
		t.Helper()
		node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
		t.Cleanup(func() { node.StopDaemon() })
		return node
	}

	t.Run("block put returns base32 CIDv1", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)
		cid := node.PipeStrToIPFS("hello", "block", "put", "--cid-base=base32").Stdout.Trimmed()
		require.True(t, strings.HasPrefix(cid, "bafk"), "expected base32 CIDv1, got %s", cid)
	})

	t.Run("block put --format=v0 --cid-base=base32 upgrades output to CIDv1", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)

		// Without --cid-base: CIDv0 in base58btc
		cidV0 := node.PipeStrToIPFS("hello", "block", "put", "--format=v0").Stdout.Trimmed()
		require.True(t, strings.HasPrefix(cidV0, "Qm"), "expected CIDv0, got %s", cidV0)

		// With --cid-base=base32: same content but displayed as CIDv1
		cidV1 := node.PipeStrToIPFS("hello", "block", "put", "--format=v0", "--cid-base=base32").Stdout.Trimmed()
		require.True(t, strings.HasPrefix(cidV1, "bafy"), "expected base32 CIDv1, got %s", cidV1)
	})

	t.Run("block stat respects --cid-base", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)

		// Add a CIDv0 block
		cidV0 := node.PipeStrToIPFS("test-block-stat", "block", "put", "--format=v0").Stdout.Trimmed()
		require.True(t, strings.HasPrefix(cidV0, "Qm"))

		// block stat without --cid-base returns CIDv0
		stat := node.IPFS("block", "stat", cidV0).Stdout.Trimmed()
		require.Contains(t, stat, cidV0)

		// block stat with --cid-base=base32 returns CIDv1
		stat = node.IPFS("block", "stat", "--cid-base=base32", cidV0).Stdout.Trimmed()
		require.NotContains(t, stat, cidV0, "should not contain CIDv0")
		require.Contains(t, stat, "bafy", "should contain base32 CIDv1")
	})

	t.Run("block rm respects --cid-base", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)

		cidV0 := node.PipeStrToIPFS("test-block-rm", "block", "put", "--format=v0").Stdout.Trimmed()
		require.True(t, strings.HasPrefix(cidV0, "Qm"))

		out := node.IPFS("block", "rm", "--cid-base=base32", cidV0).Stdout.Trimmed()
		require.Contains(t, out, "bafy", "removed block should be shown as base32 CIDv1")
		require.NotContains(t, out, "Qm", "removed block should not contain CIDv0")
	})

	t.Run("dag stat respects --cid-base", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)

		// ipfs add creates dag-pb blocks with CIDv0 by default
		cidV0 := node.IPFSAddStr("test-dag-stat", "--pin=false")
		require.True(t, strings.HasPrefix(cidV0, "Qm"))

		// JSON output without --cid-base has CIDv0
		out := node.IPFS("dag", "stat", "--progress=false", "--enc=json", cidV0).Stdout.Trimmed()
		var data struct {
			DagStats []struct{ Cid string } `json:"DagStats"`
		}
		require.NoError(t, json.Unmarshal([]byte(out), &data))
		require.True(t, strings.HasPrefix(data.DagStats[0].Cid, "Qm"))

		// JSON output with --cid-base=base32 has CIDv1
		out = node.IPFS("dag", "stat", "--progress=false", "--enc=json", "--cid-base=base32", cidV0).Stdout.Trimmed()
		require.NoError(t, json.Unmarshal([]byte(out), &data))
		require.True(t, strings.HasPrefix(data.DagStats[0].Cid, "bafy"), "expected base32 CIDv1 in dag stat, got %s", data.DagStats[0].Cid)
	})

	t.Run("object patch add-link respects --cid-base", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)

		parent := node.IPFSAddStr("parent", "--pin=false")
		child := node.IPFSAddStr("child", "--pin=false")

		// Without --cid-base: CIDv0
		cidV0 := node.IPFS("object", "patch", "add-link", parent, "link", child).Stdout.Trimmed()
		require.True(t, strings.HasPrefix(cidV0, "Qm"), "expected CIDv0, got %s", cidV0)

		// With --cid-base=base32: CIDv1
		cidV1 := node.IPFS("object", "patch", "add-link", "--cid-base=base32", parent, "link", child).Stdout.Trimmed()
		require.True(t, strings.HasPrefix(cidV1, "bafy"), "expected base32 CIDv1, got %s", cidV1)
	})

	t.Run("object patch rm-link respects --cid-base", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)

		parent := node.IPFSAddStr("parent", "--pin=false")
		child := node.IPFSAddStr("child", "--pin=false")

		// Create a linked object first
		linked := node.IPFS("object", "patch", "add-link", parent, "link", child).Stdout.Trimmed()

		// rm-link with --cid-base=base32
		cidV1 := node.IPFS("object", "patch", "rm-link", "--cid-base=base32", linked, "link").Stdout.Trimmed()
		require.True(t, strings.HasPrefix(cidV1, "bafy"), "expected base32 CIDv1, got %s", cidV1)
	})
}
