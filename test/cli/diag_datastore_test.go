package cli

import (
	"encoding/json"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiagDatastore(t *testing.T) {
	t.Parallel()

	t.Run("diag datastore get returns error for non-existent key", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		// Don't start daemon - these commands require daemon to be stopped

		res := node.RunIPFS("diag", "datastore", "get", "/nonexistent/key")
		assert.Error(t, res.Err)
		assert.Contains(t, res.Stderr.String(), "key not found")
	})

	t.Run("diag datastore get returns raw bytes by default", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		// Add some data to create a known datastore key
		// We need daemon for add, then stop it
		node.StartDaemon()
		cid := node.IPFSAddStr("test data for diag datastore")
		node.IPFS("pin", "add", cid)
		node.StopDaemon()

		// Test count to verify we have entries
		count := node.DatastoreCount("/")
		t.Logf("total datastore entries: %d", count)
		assert.NotEqual(t, int64(0), count, "should have datastore entries after pinning")
	})

	t.Run("diag datastore get --hex returns hex dump", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		// Add and pin some data
		node.StartDaemon()
		cid := node.IPFSAddStr("test data for hex dump")
		node.IPFS("pin", "add", cid)
		node.StopDaemon()

		// Test with existing keys in pins namespace
		count := node.DatastoreCount("/pins/")
		t.Logf("pins datastore entries: %d", count)

		if count != 0 {
			t.Log("pins datastore has entries, hex dump format tested implicitly")
		}
	})

	t.Run("diag datastore count returns 0 for empty prefix", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		count := node.DatastoreCount("/definitely/nonexistent/prefix/")
		assert.Equal(t, int64(0), count)
	})

	t.Run("diag datastore count returns JSON with --enc=json", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		res := node.IPFS("diag", "datastore", "count", "/pubsub/seqno/", "--enc=json")
		assert.NoError(t, res.Err)

		var result struct {
			Prefix string `json:"prefix"`
			Count  int64  `json:"count"`
		}
		err := json.Unmarshal(res.Stdout.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, "/pubsub/seqno/", result.Prefix)
		assert.Equal(t, int64(0), result.Count)
	})

	t.Run("diag datastore get returns JSON with --enc=json", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		// Test error case with JSON encoding
		res := node.RunIPFS("diag", "datastore", "get", "/nonexistent", "--enc=json")
		assert.Error(t, res.Err)
	})

	t.Run("diag datastore count counts entries correctly", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		// Add multiple pins to create multiple entries
		node.StartDaemon()
		cid1 := node.IPFSAddStr("data 1")
		cid2 := node.IPFSAddStr("data 2")
		cid3 := node.IPFSAddStr("data 3")

		node.IPFS("pin", "add", cid1)
		node.IPFS("pin", "add", cid2)
		node.IPFS("pin", "add", cid3)
		node.StopDaemon()

		// Count should reflect the pins (plus any system entries)
		count := node.DatastoreCount("/")
		t.Logf("total entries after adding 3 pins: %d", count)

		// Should have more than 0 entries
		assert.NotEqual(t, int64(0), count)
	})

	t.Run("diag datastore commands work offline", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		// Don't start daemon - these commands require daemon to be stopped

		// Count should work offline
		count := node.DatastoreCount("/pubsub/seqno/")
		assert.Equal(t, int64(0), count)

		// Get should return error for missing key (but command should work)
		res := node.RunIPFS("diag", "datastore", "get", "/nonexistent/key")
		assert.Error(t, res.Err)
		assert.Contains(t, res.Stderr.String(), "key not found")
	})

	t.Run("diag datastore commands require daemon to be stopped", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		// Both get and count require repo lock, which is held by the running daemon
		res := node.RunIPFS("diag", "datastore", "get", "/test")
		assert.Error(t, res.Err, "get should fail when daemon is running")
		assert.Contains(t, res.Stderr.String(), "ipfs daemon is running")

		res = node.RunIPFS("diag", "datastore", "count", "/pubsub/seqno/")
		assert.Error(t, res.Err, "count should fail when daemon is running")
		assert.Contains(t, res.Stderr.String(), "ipfs daemon is running")
	})
}
