package cli

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	twoMiB       = 2 * 1024 * 1024  // 2097152 - bitswap spec block size limit
	twoMiBPlus   = twoMiB + 1       // 2097153
	maxChunkSize = twoMiB - 256     // 2096896 - max chunker value (overhead budget for protobuf framing)
	overMaxChunk = maxChunkSize + 1 // 2096897

	// go-libp2p v0.47.0 network.MessageSizeMax is 4194304 bytes (4MiB).
	// A bitswap message carrying a single block has a protobuf envelope
	// whose size depends on the CID used to represent the block. For
	// CIDv1 with raw codec and SHA2-256 multihash (4-byte CID prefix),
	// the envelope is 18 bytes: 2 bytes for the empty Wantlist submessage,
	// 6 bytes for the CID prefix field, 5 bytes for field tags and the
	// payload length varint, and 5 bytes for the data length varint and
	// block submessage length varint. The msgio varint reader rejects
	// messages strictly larger than MessageSizeMax, so the maximum block
	// that fits is 4194304 - 18 = 4194286 bytes.
	//
	// The hard limit varies slightly depending on the CID: a longer
	// multihash (e.g. SHA-512) increases the CID prefix and reduces the
	// maximum block payload by the same amount.
	libp2pMsgMax     = 4 * 1024 * 1024                // 4194304 - libp2p network.MessageSizeMax
	bsBlockEnvelope  = 18                             // protobuf overhead for CIDv1 + raw + SHA2-256
	maxTransferBlock = libp2pMsgMax - bsBlockEnvelope // 4194286 - largest block transferable via bitswap
	overMaxTransfer  = maxTransferBlock + 1           // 4194287
)

// blockSize returns the block size in bytes for a given CID by parsing
// the JSON output of `ipfs block stat --enc=json <cid>`.
func blockSize(t *testing.T, node *harness.Node, cid string) int {
	t.Helper()
	res := node.IPFS("block", "stat", "--enc=json", cid)
	var stat struct {
		Key  string
		Size int
	}
	require.NoError(t, json.Unmarshal(res.Stdout.Bytes(), &stat))
	return stat.Size
}

// allBlockCIDs returns the root CID plus all recursive refs for a DAG.
func allBlockCIDs(t *testing.T, node *harness.Node, root string) []string {
	t.Helper()
	cids := []string{root}
	res := node.IPFS("refs", "-r", "--unique", root)
	for line := range strings.SplitSeq(strings.TrimSpace(res.Stdout.String()), "\n") {
		if line != "" {
			cids = append(cids, line)
		}
	}
	return cids
}

// assertAllBlocksWithinLimit checks that every block in the DAG rooted at
// root is at most twoMiB bytes.
func assertAllBlocksWithinLimit(t *testing.T, node *harness.Node, root string) {
	t.Helper()
	for _, c := range allBlockCIDs(t, node, root) {
		size := blockSize(t, node, c)
		assert.LessOrEqual(t, size, twoMiB, fmt.Sprintf("block %s is %d bytes, exceeds 2MiB limit", c, size))
	}
}

func TestBlockSizeBoundary(t *testing.T) {
	t.Parallel()

	t.Run("block put", func(t *testing.T) {
		t.Parallel()

		t.Run("exactly 2MiB succeeds", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
			defer node.StopDaemon()

			data := make([]byte, twoMiB)
			cid := strings.TrimSpace(
				node.PipeToIPFS(bytes.NewReader(data), "block", "put").Stdout.String(),
			)
			got := node.IPFS("block", "get", cid)
			assert.Len(t, got.Stdout.Bytes(), twoMiB)
		})

		t.Run("2MiB+1 fails without --allow-big-block", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
			defer node.StopDaemon()

			data := make([]byte, twoMiBPlus)
			res := node.RunPipeToIPFS(bytes.NewReader(data), "block", "put")
			assert.NotEqual(t, 0, res.ExitCode())
			assert.Contains(t, res.Stderr.String(), "produced block is over 2MiB: big blocks can't be exchanged with other peers. consider using UnixFS for automatic chunking of bigger files, or pass --allow-big-block to override")
		})

		t.Run("2MiB+1 succeeds with --allow-big-block", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
			defer node.StopDaemon()

			data := make([]byte, twoMiBPlus)
			cid := strings.TrimSpace(
				node.PipeToIPFS(bytes.NewReader(data), "block", "put", "--allow-big-block").Stdout.String(),
			)
			got := node.IPFS("block", "get", cid)
			assert.Len(t, got.Stdout.Bytes(), twoMiBPlus)
		})
	})

	t.Run("dag put", func(t *testing.T) {
		t.Parallel()

		t.Run("exactly 2MiB succeeds", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
			defer node.StopDaemon()

			data := make([]byte, twoMiB)
			cid := strings.TrimSpace(
				node.PipeToIPFS(bytes.NewReader(data), "dag", "put", "--input-codec=raw", "--store-codec=raw").Stdout.String(),
			)
			got := node.IPFS("block", "get", cid)
			assert.Len(t, got.Stdout.Bytes(), twoMiB)
		})

		t.Run("2MiB+1 fails without --allow-big-block", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
			defer node.StopDaemon()

			data := make([]byte, twoMiBPlus)
			res := node.RunPipeToIPFS(bytes.NewReader(data), "dag", "put", "--input-codec=raw", "--store-codec=raw")
			assert.NotEqual(t, 0, res.ExitCode())
			assert.Contains(t, res.Stderr.String(), "produced block is over 2MiB: big blocks can't be exchanged with other peers. consider using UnixFS for automatic chunking of bigger files, or pass --allow-big-block to override")
		})

		t.Run("2MiB+1 succeeds with --allow-big-block", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
			defer node.StopDaemon()

			data := make([]byte, twoMiBPlus)
			cid := strings.TrimSpace(
				node.PipeToIPFS(bytes.NewReader(data), "dag", "put", "--input-codec=raw", "--store-codec=raw", "--allow-big-block").Stdout.String(),
			)
			got := node.IPFS("block", "get", cid)
			assert.Len(t, got.Stdout.Bytes(), twoMiBPlus)
		})
	})

	t.Run("dag import and export", func(t *testing.T) {
		t.Parallel()

		t.Run("2MiB+1 block round-trips with --allow-big-block", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
			defer node.StopDaemon()

			// put an oversized raw block with override
			data := make([]byte, twoMiBPlus)
			cid := strings.TrimSpace(
				node.PipeToIPFS(bytes.NewReader(data), "dag", "put", "--input-codec=raw", "--store-codec=raw", "--allow-big-block").Stdout.String(),
			)

			// export to CAR
			carPath := filepath.Join(node.Dir, "oversized.car")
			require.NoError(t, node.IPFSDagExport(cid, carPath))

			// re-import without --allow-big-block should fail
			carFile, err := os.Open(carPath)
			require.NoError(t, err)
			res := node.RunPipeToIPFS(carFile, "dag", "import")
			carFile.Close()
			assert.NotEqual(t, 0, res.ExitCode())
			assert.Contains(t, res.Stderr.String()+res.Stdout.String(), "produced block is over 2MiB: big blocks can't be exchanged with other peers. consider using UnixFS for automatic chunking of bigger files, or pass --allow-big-block to override")

			// re-import with --allow-big-block should succeed
			carFile, err = os.Open(carPath)
			require.NoError(t, err)
			res = node.RunPipeToIPFS(carFile, "dag", "import", "--allow-big-block")
			carFile.Close()
			assert.Equal(t, 0, res.ExitCode())
		})
	})

	t.Run("ipfs add non-raw-leaves", func(t *testing.T) {
		t.Parallel()

		// The chunker enforces ChunkSizeLimit (maxChunkSize = 2MiB - 256
		// as of boxo 2026Q1) regardless of leaf type. It does not know at parse time whether
		// raw or wrapped leaves will be used, so the 256-byte overhead
		// budget is applied uniformly.
		//
		// With --raw-leaves=false each chunk is wrapped in protobuf,
		// adding ~14 bytes overhead that pushes blocks past the chunk size.
		// The overhead budget ensures the wrapped block stays within 2MiB.
		//
		// With --raw-leaves=true there is no protobuf wrapper, so the
		// block is exactly the chunk size (maxChunkSize). The 256-byte
		// budget is unused in this case but the chunker still enforces it.
		// A full 2MiB chunk (--chunker=size-2097152) is rejected even
		// though the resulting raw block would fit within BlockSizeLimit.

		t.Run("1MiB chunk with protobuf wrapping succeeds under 2MiB limit", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
			defer node.StopDaemon()

			data := make([]byte, twoMiB)
			res := node.RunPipeToIPFS(bytes.NewReader(data), "add", "-q", "--chunker=size-1048576", "--raw-leaves=false")
			require.Equal(t, 0, res.ExitCode(), "stderr: %s", res.Stderr.String())
			root := strings.TrimSpace(res.Stdout.String())
			// the last line of `ipfs add -q` is the root CID
			lines := strings.Split(root, "\n")
			root = lines[len(lines)-1]
			assertAllBlocksWithinLimit(t, node, root)
		})

		t.Run("max chunk with protobuf wrapping stays within block limit", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
			defer node.StopDaemon()

			// maxChunkSize leaves room for protobuf framing overhead
			data := make([]byte, maxChunkSize*2)
			res := node.RunPipeToIPFS(bytes.NewReader(data), "add", "-q",
				fmt.Sprintf("--chunker=size-%d", maxChunkSize), "--raw-leaves=false")
			require.Equal(t, 0, res.ExitCode(), "stderr: %s", res.Stderr.String())
			lines := strings.Split(strings.TrimSpace(res.Stdout.String()), "\n")
			root := lines[len(lines)-1]
			assertAllBlocksWithinLimit(t, node, root)
		})

		t.Run("chunk size over limit is rejected by chunker", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
			defer node.StopDaemon()

			data := make([]byte, twoMiB+twoMiB)
			res := node.RunPipeToIPFS(bytes.NewReader(data), "add", "-q",
				fmt.Sprintf("--chunker=size-%d", overMaxChunk), "--raw-leaves=false")
			assert.NotEqual(t, 0, res.ExitCode())
			assert.Contains(t, res.Stderr.String(),
				fmt.Sprintf("chunker parameters may not exceed the maximum chunk size of %d", maxChunkSize))
		})

		t.Run("max chunk with raw leaves succeeds", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
			defer node.StopDaemon()

			// raw leaves have no protobuf wrapper, so max chunk size fits easily
			data := make([]byte, maxChunkSize*2)
			res := node.RunPipeToIPFS(bytes.NewReader(data), "add", "-q",
				fmt.Sprintf("--chunker=size-%d", maxChunkSize), "--raw-leaves=true")
			require.Equal(t, 0, res.ExitCode(), "stderr: %s", res.Stderr.String())
			lines := strings.Split(strings.TrimSpace(res.Stdout.String()), "\n")
			root := lines[len(lines)-1]
			assertAllBlocksWithinLimit(t, node, root)
		})
	})

	t.Run("bitswap exchange", func(t *testing.T) {
		t.Parallel()

		t.Run("2MiB raw block transfers between peers", func(t *testing.T) {
			t.Parallel()
			h := harness.NewT(t)
			provider := h.NewNode().Init("--profile=unixfs-v1-2025").StartDaemon()
			defer provider.StopDaemon()
			requester := h.NewNode().Init("--profile=unixfs-v1-2025").StartDaemon()
			defer requester.StopDaemon()

			data := make([]byte, twoMiB)
			_, err := rand.Read(data)
			require.NoError(t, err)
			cid := strings.TrimSpace(
				provider.PipeToIPFS(bytes.NewReader(data), "block", "put").Stdout.String(),
			)

			requester.Connect(provider)

			res := requester.IPFS("block", "get", cid)
			assert.Equal(t, data, res.Stdout.Bytes(), "retrieved block should match original")
		})

		t.Run("unixfs-v1-2025: 2MiB file transfers between peers", func(t *testing.T) {
			t.Parallel()
			h := harness.NewT(t)
			provider := h.NewNode().Init("--profile=unixfs-v1-2025").StartDaemon()
			defer provider.StopDaemon()
			requester := h.NewNode().Init("--profile=unixfs-v1-2025").StartDaemon()
			defer requester.StopDaemon()

			// unixfs-v1-2025 profile uses CIDv1, raw leaves, SHA2-256,
			// and 1MiB chunks. A 2MiB file produces two 1MiB raw leaf
			// blocks plus a root node, all within the 2MiB spec limit.
			data := make([]byte, twoMiB)
			_, err := rand.Read(data)
			require.NoError(t, err)
			res := provider.RunPipeToIPFS(bytes.NewReader(data), "add", "-q")
			require.Equal(t, 0, res.ExitCode(), "stderr: %s", res.Stderr.String())
			lines := strings.Split(strings.TrimSpace(res.Stdout.String()), "\n")
			root := lines[len(lines)-1]

			requester.Connect(provider)

			got := requester.IPFS("cat", root)
			assert.Equal(t, data, got.Stdout.Bytes(), "retrieved file should match original")
		})

		// The following two tests guard the physical hard limit of the
		// libp2p transport layer (network.MessageSizeMax = 4MiB). This is
		// the actual ceiling for bitswap block transfer, independent of the
		// 2MiB soft limit from the bitswap spec. Knowing the exact hard
		// limit is important for backward-compatible protocol and standards
		// evolution: any future increase to the bitswap spec block size
		// must stay within the libp2p message framing budget, or the
		// transport layer must be updated first.

		t.Run("bitswap-over-libp2p: largest block that fits in message transfers", func(t *testing.T) {
			t.Parallel()
			h := harness.NewT(t)
			provider := h.NewNode().Init("--profile=unixfs-v1-2025").StartDaemon()
			defer provider.StopDaemon()
			requester := h.NewNode().Init("--profile=unixfs-v1-2025").StartDaemon()
			defer requester.StopDaemon()

			data := make([]byte, maxTransferBlock)
			_, err := rand.Read(data)
			require.NoError(t, err)
			cid := strings.TrimSpace(
				provider.PipeToIPFS(bytes.NewReader(data), "block", "put", "--allow-big-block").Stdout.String(),
			)

			requester.Connect(provider)

			// successful transfers complete in ~1s
			timeout := time.After(5 * time.Second)
			dataChan := make(chan []byte, 1)

			go func() {
				res := requester.RunIPFS("block", "get", cid)
				dataChan <- res.Stdout.Bytes()
			}()

			select {
			case got := <-dataChan:
				assert.Equal(t, data, got, "retrieved block should match original")
			case <-timeout:
				t.Fatal("block get timed out: expected transfer to succeed at maxTransferBlock")
			}
		})

		t.Run("bitswap-over-libp2p: one byte over message limit does not transfer", func(t *testing.T) {
			t.Parallel()
			h := harness.NewT(t)
			provider := h.NewNode().Init("--profile=unixfs-v1-2025").StartDaemon()
			defer provider.StopDaemon()
			requester := h.NewNode().Init("--profile=unixfs-v1-2025").StartDaemon()
			defer requester.StopDaemon()

			data := make([]byte, overMaxTransfer)
			_, err := rand.Read(data)
			require.NoError(t, err)
			cid := strings.TrimSpace(
				provider.PipeToIPFS(bytes.NewReader(data), "block", "put", "--allow-big-block").Stdout.String(),
			)

			requester.Connect(provider)

			timeout := time.After(5 * time.Second)
			dataChan := make(chan []byte, 1)

			go func() {
				res := requester.RunIPFS("block", "get", cid)
				dataChan <- res.Stdout.Bytes()
			}()

			select {
			case got := <-dataChan:
				t.Fatalf("expected timeout, but block was retrieved (%d bytes)", len(got))
			case <-timeout:
				t.Log("block get timed out as expected: block exceeds libp2p message size limit")
			}
		})
	})
}
