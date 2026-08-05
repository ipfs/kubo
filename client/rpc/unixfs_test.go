package rpc

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ipfs/boxo/files"
	caopts "github.com/ipfs/kubo/core/coreiface/options"
	"github.com/ipfs/kubo/test/cli/harness"
	mh "github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"
)

// TestAddFollowsDaemonImportConfig checks that an Add over the HTTP RPC returns
// the same CID as `ipfs add` on the same daemon. The client must not send its
// own defaults for anything that shapes the DAG (CID version, leaves, chunker,
// hash, layout), or a program using client/rpc gets different CIDs than the CLI
// on a node whose Import config differs from those defaults.
// Deliberately not parallel: TestHttpApi in this package runs a DHT swarm whose
// peer lookups time out when this test's adds compete with it for the machine.
func TestAddFollowsDaemonImportConfig(t *testing.T) {
	node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
	defer node.StopDaemon()

	apiMaddr, err := node.TryAPIAddr()
	require.NoError(t, err)
	api, err := NewApi(apiMaddr)
	require.NoError(t, err)

	// 2 MiB spans several chunks under the 1 MiB chunker of unixfs-v1-2025 and
	// many more under the 256 KiB default the client used to send, so the two
	// disagree on the root CID unless the client defers to the daemon.
	content := strings.Repeat("kubo", 512*1024)

	t.Run("client defaults defer to the daemon", func(t *testing.T) {
		cliCid := node.PipeStrToIPFS(content, "add", "-q", "--pin=false").Stdout.Trimmed()

		p, err := api.Unixfs().Add(t.Context(), files.NewBytesFile([]byte(content)), caopts.Unixfs.Pin(false, ""))
		require.NoError(t, err)
		require.Equal(t, cliCid, p.RootCid().String())
	})

	t.Run("explicit client options still win", func(t *testing.T) {
		cliCid := node.PipeStrToIPFS(content, "add", "-q", "--pin=false",
			"--chunker=size-262144", "--cid-version=0").Stdout.Trimmed()

		p, err := api.Unixfs().Add(t.Context(), files.NewBytesFile([]byte(content)),
			caopts.Unixfs.Pin(false, ""),
			caopts.Unixfs.Chunker("size-262144"),
			caopts.Unixfs.CidVersion(0))
		require.NoError(t, err)
		require.Equal(t, cliCid, p.RootCid().String())
		require.True(t, strings.HasPrefix(cliCid, "Qm"), "expected a CIDv0 root, got %s", cliCid)
	})

	t.Run("trickle layout is forwarded", func(t *testing.T) {
		cliCid := node.PipeStrToIPFS(content, "add", "-q", "--pin=false", "--trickle").Stdout.Trimmed()

		p, err := api.Unixfs().Add(t.Context(), files.NewBytesFile([]byte(content)),
			caopts.Unixfs.Pin(false, ""),
			caopts.Unixfs.Layout(caopts.TrickleLayout))
		require.NoError(t, err)
		require.Equal(t, cliCid, p.RootCid().String())
	})

	t.Run("hash is forwarded", func(t *testing.T) {
		cliCid := node.PipeStrToIPFS(content, "add", "-q", "--pin=false", "--hash=blake3").Stdout.Trimmed()

		p, err := api.Unixfs().Add(t.Context(), files.NewBytesFile([]byte(content)),
			caopts.Unixfs.Pin(false, ""),
			caopts.Unixfs.Hash(mh.BLAKE3))
		require.NoError(t, err)
		require.Equal(t, cliCid, p.RootCid().String())
	})

	t.Run("added content is readable back", func(t *testing.T) {
		p, err := api.Unixfs().Add(t.Context(), files.NewBytesFile([]byte(content)), caopts.Unixfs.Pin(false, ""))
		require.NoError(t, err)

		nd, err := api.Unixfs().Get(t.Context(), p)
		require.NoError(t, err)
		f, ok := nd.(files.File)
		require.True(t, ok)
		var buf bytes.Buffer
		_, err = buf.ReadFrom(f)
		require.NoError(t, err)
		require.Equal(t, content, buf.String())
	})
}
