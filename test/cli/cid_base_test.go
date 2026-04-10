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
//
// Tests use base16 rather than base32 to avoid false positives if
// base32 ever becomes the default CID encoding.
func TestCidBase(t *testing.T) {
	t.Parallel()

	const cidBaseFlag = "--cid-base=base16"
	// base16 CIDv1 starts with "f01" (f = base16 multibase prefix)
	const cidV1Prefix = "f01"

	makeDaemon := func(t *testing.T) *harness.Node {
		t.Helper()
		node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
		t.Cleanup(func() { node.StopDaemon() })
		return node
	}

	t.Run("block put returns base16 CIDv1", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)
		cid := node.PipeStrToIPFS("hello", "block", "put", cidBaseFlag).Stdout.Trimmed()
		require.True(t, strings.HasPrefix(cid, cidV1Prefix), "expected base16 CIDv1, got %s", cid)
	})

	t.Run("block put --format=v0 auto-upgrades to CIDv1 with --cid-base", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)

		// Without --cid-base: CIDv0 in base58btc
		cidV0 := node.PipeStrToIPFS("hello", "block", "put", "--format=v0").Stdout.Trimmed()
		require.True(t, strings.HasPrefix(cidV0, "Qm"), "expected CIDv0, got %s", cidV0)

		// With --cid-base: same content but displayed as CIDv1
		cidV1 := node.PipeStrToIPFS("hello", "block", "put", "--format=v0", cidBaseFlag).Stdout.Trimmed()
		require.True(t, strings.HasPrefix(cidV1, cidV1Prefix), "expected base16 CIDv1, got %s", cidV1)
	})

	t.Run("block stat respects --cid-base", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)

		cidV0 := node.PipeStrToIPFS("test-block-stat", "block", "put", "--format=v0").Stdout.Trimmed()
		require.True(t, strings.HasPrefix(cidV0, "Qm"))

		// block stat without --cid-base returns CIDv0
		stat := node.IPFS("block", "stat", cidV0).Stdout.Trimmed()
		require.Contains(t, stat, cidV0)

		// block stat with --cid-base returns CIDv1
		stat = node.IPFS("block", "stat", cidBaseFlag, cidV0).Stdout.Trimmed()
		require.NotContains(t, stat, cidV0, "should not contain CIDv0")
		require.Contains(t, stat, cidV1Prefix, "should contain base16 CIDv1")
	})

	t.Run("block rm respects --cid-base", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)

		cidV0 := node.PipeStrToIPFS("test-block-rm", "block", "put", "--format=v0").Stdout.Trimmed()
		require.True(t, strings.HasPrefix(cidV0, "Qm"))

		out := node.IPFS("block", "rm", cidBaseFlag, cidV0).Stdout.Trimmed()
		require.Contains(t, out, cidV1Prefix, "removed block should be shown as base16 CIDv1")
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

		// JSON output with --cid-base has CIDv1
		out = node.IPFS("dag", "stat", "--progress=false", "--enc=json", cidBaseFlag, cidV0).Stdout.Trimmed()
		require.NoError(t, json.Unmarshal([]byte(out), &data))
		require.True(t, strings.HasPrefix(data.DagStats[0].Cid, cidV1Prefix), "expected base16 CIDv1 in dag stat, got %s", data.DagStats[0].Cid)
	})

	t.Run("object patch add-link respects --cid-base", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)

		parent := node.IPFSAddStr("parent", "--pin=false")
		child := node.IPFSAddStr("child", "--pin=false")

		// Without --cid-base: CIDv0
		cidV0 := node.IPFS("object", "patch", "add-link", parent, "link", child).Stdout.Trimmed()
		require.True(t, strings.HasPrefix(cidV0, "Qm"), "expected CIDv0, got %s", cidV0)

		// With --cid-base: CIDv1
		cidV1 := node.IPFS("object", "patch", "add-link", cidBaseFlag, parent, "link", child).Stdout.Trimmed()
		require.True(t, strings.HasPrefix(cidV1, cidV1Prefix), "expected base16 CIDv1, got %s", cidV1)
	})

	t.Run("object patch rm-link respects --cid-base", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)

		parent := node.IPFSAddStr("parent", "--pin=false")
		child := node.IPFSAddStr("child", "--pin=false")

		linked := node.IPFS("object", "patch", "add-link", parent, "link", child).Stdout.Trimmed()

		cidV1 := node.IPFS("object", "patch", "rm-link", cidBaseFlag, linked, "link").Stdout.Trimmed()
		require.True(t, strings.HasPrefix(cidV1, cidV1Prefix), "expected base16 CIDv1, got %s", cidV1)
	})

	t.Run("refs local respects --cid-base", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)

		node.IPFSAddStr("refs-local-test", "--pin=false")

		lines := node.IPFS("refs", "local", cidBaseFlag).Stdout.Lines()
		for _, line := range lines {
			if line == "" {
				continue
			}
			require.True(t, strings.HasPrefix(line, cidV1Prefix), "expected base16 CID, got %s", line)
		}
	})

	t.Run("object diff respects --cid-base", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)

		cidA := node.IPFSAddStr("aaa", "--pin=false")
		cidB := node.IPFSAddStr("bbb", "--pin=false")

		// Create two directories with different children
		node.IPFS("files", "mkdir", "/diff-a")
		node.IPFS("files", "cp", "/ipfs/"+cidA, "/diff-a/file")
		dirA := node.IPFS("files", "stat", "--hash", "/diff-a").Stdout.Trimmed()

		node.IPFS("files", "mkdir", "/diff-b")
		node.IPFS("files", "cp", "/ipfs/"+cidB, "/diff-b/file")
		dirB := node.IPFS("files", "stat", "--hash", "/diff-b").Stdout.Trimmed()

		// Without --cid-base: CIDs in diff output are CIDv0
		out := node.IPFS("object", "diff", dirA, dirB).Stdout.Trimmed()
		require.Contains(t, out, "Qm")

		// With --cid-base: CIDs in diff output should be base16
		out = node.IPFS("object", "diff", cidBaseFlag, dirA, dirB).Stdout.Trimmed()
		require.Contains(t, out, cidV1Prefix, "expected base16 CIDs in diff output")
		require.NotContains(t, out, "Qm", "should not contain CIDv0 in diff output")
	})
}
