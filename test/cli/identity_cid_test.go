package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ipfs/boxo/verifcid"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentityCIDOverflowProtection(t *testing.T) {
	t.Parallel()

	t.Run("ipfs add --hash=identity with small data succeeds", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		// small data that fits in identity CID
		smallData := "small data"
		tempFile := filepath.Join(node.Dir, "small.txt")
		err := os.WriteFile(tempFile, []byte(smallData), 0644)
		require.NoError(t, err)

		res := node.IPFS("add", "--hash=identity", tempFile)
		assert.NoError(t, res.Err)
		cid := strings.Fields(res.Stdout.String())[1]

		// verify it's actually using identity hash
		res = node.IPFS("cid", "format", "-f", "%h", cid)
		assert.NoError(t, res.Err)
		assert.Equal(t, "identity", res.Stdout.Trimmed())
	})

	t.Run("ipfs add --hash=identity with large data fails", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		// data larger than verifcid.DefaultMaxIdentityDigestSize
		largeData := strings.Repeat("x", verifcid.DefaultMaxIdentityDigestSize+50)
		tempFile := filepath.Join(node.Dir, "large.txt")
		err := os.WriteFile(tempFile, []byte(largeData), 0644)
		require.NoError(t, err)

		res := node.RunIPFS("add", "--hash=identity", tempFile)
		assert.NotEqual(t, 0, res.ExitErr.ExitCode())
		// should error with digest too large message
		assert.Contains(t, res.Stderr.String(), "digest too large")
	})

	t.Run("ipfs add --inline with valid --inline-limit succeeds", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		smallData := "small inline data"
		tempFile := filepath.Join(node.Dir, "inline.txt")
		err := os.WriteFile(tempFile, []byte(smallData), 0644)
		require.NoError(t, err)

		// use limit just under the maximum
		limit := verifcid.DefaultMaxIdentityDigestSize - 10
		res := node.IPFS("add", "--inline", fmt.Sprintf("--inline-limit=%d", limit), tempFile)
		assert.NoError(t, res.Err)
		cid := strings.Fields(res.Stdout.String())[1]

		// verify the CID is using identity hash (inline)
		res = node.IPFS("cid", "format", "-f", "%h", cid)
		assert.NoError(t, res.Err)
		assert.Equal(t, "identity", res.Stdout.Trimmed())

		// verify the codec (may be dag-pb or raw depending on kubo version)
		res = node.IPFS("cid", "format", "-f", "%c", cid)
		assert.NoError(t, res.Err)
		// Accept either raw or dag-pb as both are valid for inline data
		codec := res.Stdout.Trimmed()
		assert.True(t, codec == "raw" || codec == "dag-pb", "expected raw or dag-pb codec, got %s", codec)
	})

	t.Run("ipfs add --inline with excessive --inline-limit fails", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		smallData := "data"
		tempFile := filepath.Join(node.Dir, "inline2.txt")
		err := os.WriteFile(tempFile, []byte(smallData), 0644)
		require.NoError(t, err)

		excessiveLimit := verifcid.DefaultMaxIdentityDigestSize + 50
		res := node.RunIPFS("add", "--inline", fmt.Sprintf("--inline-limit=%d", excessiveLimit), tempFile)
		assert.NotEqual(t, 0, res.ExitErr.ExitCode())
		assert.Contains(t, res.Stderr.String(), fmt.Sprintf("inline-limit %d exceeds maximum allowed size of %d bytes", excessiveLimit, verifcid.DefaultMaxIdentityDigestSize))
	})

	t.Run("ipfs files write --hash=identity appending to identity CID switches to configured hash", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		// create initial small file with identity CID
		initialData := "initial"
		tempFile := filepath.Join(node.Dir, "initial.txt")
		err := os.WriteFile(tempFile, []byte(initialData), 0644)
		require.NoError(t, err)

		res := node.IPFS("add", "--hash=identity", tempFile)
		assert.NoError(t, res.Err)
		cid1 := strings.Fields(res.Stdout.String())[1]

		// verify initial CID uses identity
		res = node.IPFS("cid", "format", "-f", "%h", cid1)
		assert.NoError(t, res.Err)
		assert.Equal(t, "identity", res.Stdout.Trimmed())

		// copy to MFS
		res = node.IPFS("files", "cp", fmt.Sprintf("/ipfs/%s", cid1), "/identity-file")
		assert.NoError(t, res.Err)

		// append data that would exceed identity CID limit
		appendData := strings.Repeat("a", verifcid.DefaultMaxIdentityDigestSize)
		appendFile := filepath.Join(node.Dir, "append.txt")
		err = os.WriteFile(appendFile, []byte(appendData), 0644)
		require.NoError(t, err)

		// append to the end of the file
		// get the current data size
		res = node.IPFS("files", "stat", "--format", "<size>", "/identity-file")
		assert.NoError(t, res.Err)
		size := res.Stdout.Trimmed()
		// this should succeed because DagModifier in boxo handles the overflow
		res = node.IPFS("files", "write", "--hash=identity", "--offset="+size, "/identity-file", appendFile)
		assert.NoError(t, res.Err)

		// check that the file now uses non-identity hash
		res = node.IPFS("files", "stat", "--hash", "/identity-file")
		assert.NoError(t, res.Err)
		newCid := res.Stdout.Trimmed()

		// verify new CID does NOT use identity
		res = node.IPFS("cid", "format", "-f", "%h", newCid)
		assert.NoError(t, res.Err)
		assert.NotEqual(t, "identity", res.Stdout.Trimmed())

		// verify it switched to a cryptographic hash
		assert.Equal(t, config.DefaultHashFunction, res.Stdout.Trimmed())
	})

	t.Run("ipfs files write --hash=identity with small write creates identity CID", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		// create a small file with identity hash directly in MFS
		smallData := "small"
		tempFile := filepath.Join(node.Dir, "small.txt")
		err := os.WriteFile(tempFile, []byte(smallData), 0644)
		require.NoError(t, err)

		// write to MFS with identity hash
		res := node.IPFS("files", "write", "--create", "--hash=identity", "/mfs-identity", tempFile)
		assert.NoError(t, res.Err)

		// verify using identity CID
		res = node.IPFS("files", "stat", "--hash", "/mfs-identity")
		assert.NoError(t, res.Err)
		cid := res.Stdout.Trimmed()

		// verify CID uses identity hash
		res = node.IPFS("cid", "format", "-f", "%h", cid)
		assert.NoError(t, res.Err)
		assert.Equal(t, "identity", res.Stdout.Trimmed())

		// verify content
		res = node.IPFS("files", "read", "/mfs-identity")
		assert.NoError(t, res.Err)
		assert.Equal(t, smallData, res.Stdout.Trimmed())
	})

	t.Run("raw node with identity CID converts to UnixFS when appending", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		// create raw block with identity CID
		rawData := "raw"
		tempFile := filepath.Join(node.Dir, "raw.txt")
		err := os.WriteFile(tempFile, []byte(rawData), 0644)
		require.NoError(t, err)

		res := node.IPFS("block", "put", "--format=raw", "--mhtype=identity", tempFile)
		assert.NoError(t, res.Err)
		rawCid := res.Stdout.Trimmed()

		// verify initial CID uses identity hash and raw codec
		res = node.IPFS("cid", "format", "-f", "%h", rawCid)
		assert.NoError(t, res.Err)
		assert.Equal(t, "identity", res.Stdout.Trimmed())

		res = node.IPFS("cid", "format", "-f", "%c", rawCid)
		assert.NoError(t, res.Err)
		assert.Equal(t, "raw", res.Stdout.Trimmed())

		// copy to MFS
		res = node.IPFS("files", "cp", fmt.Sprintf("/ipfs/%s", rawCid), "/raw-identity")
		assert.NoError(t, res.Err)

		// append data
		appendData := "appended"
		appendFile := filepath.Join(node.Dir, "append-raw.txt")
		err = os.WriteFile(appendFile, []byte(appendData), 0644)
		require.NoError(t, err)

		// get current data size for appending
		res = node.IPFS("files", "stat", "--format", "<size>", "/raw-identity")
		assert.NoError(t, res.Err)
		size := res.Stdout.Trimmed()
		res = node.IPFS("files", "write", "--hash=identity", "--offset="+size, "/raw-identity", appendFile)
		assert.NoError(t, res.Err)

		// verify content
		res = node.IPFS("files", "read", "/raw-identity")
		assert.NoError(t, res.Err)
		assert.Equal(t, rawData+appendData, res.Stdout.Trimmed())

		// check that it's now a UnixFS structure (dag-pb)
		res = node.IPFS("files", "stat", "--hash", "/raw-identity")
		assert.NoError(t, res.Err)
		newCid := res.Stdout.Trimmed()

		res = node.IPFS("cid", "format", "-f", "%c", newCid)
		assert.NoError(t, res.Err)
		assert.Equal(t, "dag-pb", res.Stdout.Trimmed())

		res = node.IPFS("files", "stat", "/raw-identity")
		assert.NoError(t, res.Err)
		assert.Contains(t, res.Stdout.String(), "Type: file")
	})

	t.Run("ipfs add --inline-limit at exactly max size succeeds", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		// create small data that will be inlined
		smallData := "test data for inline"
		tempFile := filepath.Join(node.Dir, "exact.txt")
		err := os.WriteFile(tempFile, []byte(smallData), 0644)
		require.NoError(t, err)

		// exactly at the limit should succeed
		res := node.IPFS("add", "--inline", fmt.Sprintf("--inline-limit=%d", verifcid.DefaultMaxIdentityDigestSize), tempFile)
		assert.NoError(t, res.Err)
		cid := strings.Fields(res.Stdout.String())[1]

		// verify it uses identity hash (inline) since data is small enough
		res = node.IPFS("cid", "format", "-f", "%h", cid)
		assert.NoError(t, res.Err)
		assert.Equal(t, "identity", res.Stdout.Trimmed())
	})

	t.Run("ipfs add --inline-limit one byte over max fails", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		smallData := "test"
		tempFile := filepath.Join(node.Dir, "oneover.txt")
		err := os.WriteFile(tempFile, []byte(smallData), 0644)
		require.NoError(t, err)

		// one byte over should fail
		overLimit := verifcid.DefaultMaxIdentityDigestSize + 1
		res := node.RunIPFS("add", "--inline", fmt.Sprintf("--inline-limit=%d", overLimit), tempFile)
		assert.NotEqual(t, 0, res.ExitErr.ExitCode())
		assert.Contains(t, res.Stderr.String(), fmt.Sprintf("inline-limit %d exceeds maximum allowed size of %d bytes", overLimit, verifcid.DefaultMaxIdentityDigestSize))
	})

	t.Run("ipfs add --inline with data larger than limit uses configured hash", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		// data larger than inline limit
		largeData := strings.Repeat("y", 100)
		tempFile := filepath.Join(node.Dir, "toolarge.txt")
		err := os.WriteFile(tempFile, []byte(largeData), 0644)
		require.NoError(t, err)

		// set inline limit smaller than data
		res := node.IPFS("add", "--inline", "--inline-limit=50", tempFile)
		assert.NoError(t, res.Err)
		cid := strings.Fields(res.Stdout.String())[1]

		// verify it's NOT using identity hash (data too large for inline)
		res = node.IPFS("cid", "format", "-f", "%h", cid)
		assert.NoError(t, res.Err)
		assert.NotEqual(t, "identity", res.Stdout.Trimmed())

		// should use configured hash
		assert.Equal(t, config.DefaultHashFunction, res.Stdout.Trimmed())
	})
}
