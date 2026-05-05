package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ipfs/go-test/random"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	timeStep = 20 * time.Millisecond
	timeout  = 30 * time.Second
)

type cfgApplier func(*harness.Node)

// uniq appends a nanosecond timestamp to s, ensuring unique CIDs
// across test runs and parallel subtests.
func uniq(s string) string {
	return s + " " + strconv.FormatInt(time.Now().UnixNano(), 10)
}

// awaitReprovideFunc waits until at least minCIDs have been provided
// and returns the total number of CIDs provided so far. The returned
// count can be passed as minCIDs to a subsequent call to wait for the
// next reprovide cycle.
type awaitReprovideFunc func(t *testing.T, n *harness.Node, minCIDs int64) int64

func runProviderSuite(t *testing.T, sweep bool, apply cfgApplier, awaitReprovide awaitReprovideFunc) {
	t.Helper()

	initNodes := func(t *testing.T, n int, fn func(n *harness.Node)) harness.Nodes {
		h := harness.NewT(t)
		nodes := h.NewNodes(n).Init()
		nodes.ForEachPar(apply)
		nodes.ForEachPar(fn)
		h.BootstrapWithStubDHT(nodes)
		nodes = nodes.StartDaemons().Connect()
		time.Sleep(500 * time.Millisecond) // wait for DHT clients to be bootstrapped
		return nodes
	}

	initNodesWithoutStart := func(t *testing.T, n int, fn func(n *harness.Node)) harness.Nodes {
		h := harness.NewT(t)
		nodes := h.NewNodes(n).Init()
		nodes.ForEachPar(apply)
		nodes.ForEachPar(fn)
		h.BootstrapWithStubDHT(nodes)
		return nodes
	}

	expectNoProviders := func(t *testing.T, cid string, nodes ...*harness.Node) {
		for _, node := range nodes {
			res := node.IPFS("routing", "findprovs", "-n=1", cid)
			require.Empty(t, res.Stdout.String())
		}
	}

	expectProviders := func(t *testing.T, cid, expectedProvider string, nodes ...*harness.Node) {
	outerLoop:
		for _, node := range nodes {
			for i := time.Duration(0); i*timeStep < timeout; i++ {
				res := node.IPFS("routing", "findprovs", "-n=1", cid)
				if res.Stdout.Trimmed() == expectedProvider {
					continue outerLoop
				}
			}
			require.FailNowf(t, "found no providers", "expected a provider for %s", cid)
		}
	}

	t.Run("Provide.Enabled=true announces new CIDs created by ipfs add", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Enabled", true)
		})
		defer nodes.StopDaemons()

		cid := nodes[0].IPFSAddStr(time.Now().String())
		expectProviders(t, cid, nodes[0].PeerID().String(), nodes[1:]...)
	})

	t.Run("Provide.Enabled=true announces new CIDs created by ipfs add --pin=false with default strategy", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Enabled", true)
			// Default strategy is "all" which should provide even unpinned content
		})
		defer nodes.StopDaemons()

		cid := nodes[0].IPFSAddStr(time.Now().String(), "--pin=false")
		expectProviders(t, cid, nodes[0].PeerID().String(), nodes[1:]...)
	})

	t.Run("Provide.Enabled=true announces new CIDs created by ipfs block put --pin=false with default strategy", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Enabled", true)
			// Default strategy is "all" which should provide unpinned content from block put
		})
		defer nodes.StopDaemons()

		data := random.Bytes(256)
		cid := nodes[0].IPFSBlockPut(bytes.NewReader(data), "--pin=false")
		expectProviders(t, cid, nodes[0].PeerID().String(), nodes[1:]...)
	})

	t.Run("Provide.Enabled=true announces new CIDs created by ipfs dag put --pin=false with default strategy", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Enabled", true)
			// Default strategy is "all" which should provide unpinned content from dag put
		})
		defer nodes.StopDaemons()

		dagData := `{"hello": "world", "timestamp": "` + time.Now().String() + `"}`
		cid := nodes[0].IPFSDAGPut(bytes.NewReader([]byte(dagData)), "--pin=false")
		expectProviders(t, cid, nodes[0].PeerID().String(), nodes[1:]...)
	})

	t.Run("Provide.Enabled=false disables announcement of new CID from ipfs add", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Enabled", false)
		})
		defer nodes.StopDaemons()

		cid := nodes[0].IPFSAddStr(time.Now().String())
		expectNoProviders(t, cid, nodes[1:]...)
	})

	t.Run("Provide.Enabled=false disables manual announcement via RPC command", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Enabled", false)
		})
		defer nodes.StopDaemons()

		cid := nodes[0].IPFSAddStr(time.Now().String())
		res := nodes[0].RunIPFS("routing", "provide", cid)
		assert.Contains(t, res.Stderr.Trimmed(), "invalid configuration: Provide.Enabled is set to 'false'")
		assert.Equal(t, 1, res.ExitCode())

		expectNoProviders(t, cid, nodes[1:]...)
	})

	t.Run("manual provide fails when no libp2p peers and no custom HTTP router", func(t *testing.T) {
		t.Parallel()

		h := harness.NewT(t)
		node := h.NewNode().Init()
		apply(node)
		node.SetIPFSConfig("Provide.Enabled", true)
		node.StartDaemon()
		defer node.StopDaemon()

		cid := node.IPFSAddStr(time.Now().String())
		res := node.RunIPFS("routing", "provide", cid)
		assert.Contains(t, res.Stderr.Trimmed(), "cannot provide, no connected peers")
		assert.Equal(t, 1, res.ExitCode())
	})

	t.Run("manual provide succeeds via custom HTTP router when no libp2p peers", func(t *testing.T) {
		t.Parallel()

		// Create a mock HTTP server that accepts provide requests.
		// This simulates the undocumented API behavior described in
		// https://discuss.ipfs.tech/t/only-peers-found-from-dht-seem-to-be-getting-used-as-relays-so-cant-use-http-routers/19545/9
		// Note: This is NOT IPIP-378, which was not implemented.
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Accept both PUT and POST requests to /routing/v1/providers and /routing/v1/ipns
			if (r.Method == http.MethodPut || r.Method == http.MethodPost) &&
				(strings.HasPrefix(r.URL.Path, "/routing/v1/providers") || strings.HasPrefix(r.URL.Path, "/routing/v1/ipns")) {
				// Return HTTP 200 to indicate successful publishing
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer mockServer.Close()

		h := harness.NewT(t)
		node := h.NewNode().Init()
		apply(node)
		node.SetIPFSConfig("Provide.Enabled", true)
		// Configure a custom HTTP router for providing.
		// Using our mock server that will accept the provide requests.
		routingConf := map[string]any{
			"Type": "custom", // https://github.com/ipfs/kubo/blob/master/docs/delegated-routing.md#configuration-file-example
			"Methods": map[string]any{
				"provide":        map[string]any{"RouterName": "MyCustomRouter"},
				"get-ipns":       map[string]any{"RouterName": "MyCustomRouter"},
				"put-ipns":       map[string]any{"RouterName": "MyCustomRouter"},
				"find-peers":     map[string]any{"RouterName": "MyCustomRouter"},
				"find-providers": map[string]any{"RouterName": "MyCustomRouter"},
			},
			"Routers": map[string]any{
				"MyCustomRouter": map[string]any{
					"Type": "http",
					"Parameters": map[string]any{
						// Use the mock server URL
						"Endpoint": mockServer.URL,
					},
				},
			},
		}
		node.SetIPFSConfig("Routing", routingConf)
		node.StartDaemon()
		defer node.StopDaemon()

		cid := node.IPFSAddStr(time.Now().String())
		// The command should successfully provide via HTTP even without libp2p peers
		res := node.RunIPFS("routing", "provide", cid)
		assert.Empty(t, res.Stderr.String(), "Should have no errors when providing via HTTP router")
		assert.Equal(t, 0, res.ExitCode(), "Should succeed with exit code 0")
	})

	t.Run("ipfs provide once works when Provide.DHT.Interval=0", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Enabled", true)
			// No periodic reprovide schedule; provide once is the only
			// way new content reaches peers in this configuration.
			n.SetIPFSConfig("Provide.DHT.Interval", "0")
			n.SetIPFSConfig("Provide.Strategy", "roots")
		})
		defer nodes.StopDaemons()

		publisher := nodes[0]
		cid := publisher.IPFSAddStr(uniq("interval=0"), "--pin=false")
		expectNoProviders(t, cid, nodes[1:]...)

		res := publisher.RunIPFS("provide", "once", cid)
		assert.Equal(t, 0, res.ExitCode(), "provide once should succeed with Interval=0")
		expectProviders(t, cid, publisher.PeerID().String(), nodes[1:]...)
	})

	t.Run("Provide.Enabled=false disables ipfs provide once", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Enabled", false)
		})
		defer nodes.StopDaemons()

		cid := nodes[0].IPFSAddStr(time.Now().String())
		res := nodes[0].RunIPFS("provide", "once", cid)
		assert.Contains(t, res.Stderr.Trimmed(), "cannot provide: Provide.Enabled is false")
		assert.Equal(t, 1, res.ExitCode())

		expectNoProviders(t, cid, nodes[1:]...)
	})

	t.Run("ipfs provide once announces a CID and finds providers", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Enabled", true)
			// "roots" so add-time providing is skipped and we know the
			// announcement comes from `provide once`, not from ipfs add.
			n.SetIPFSConfig("Provide.Strategy", "roots")
		})
		defer nodes.StopDaemons()

		cid := nodes[0].IPFSAddStr(uniq("provide once"), "--pin=false")
		expectNoProviders(t, cid, nodes[1:]...)

		res := nodes[0].RunIPFS("provide", "once", cid)
		assert.Equal(t, 0, res.ExitCode(), "provide once should succeed")
		expectProviders(t, cid, nodes[0].PeerID().String(), nodes[1:]...)
	})

	t.Run("ipfs provide once errors when CID is not in local blockstore", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 1, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Enabled", true)
		})
		defer nodes.StopDaemons()

		// CID for content the node has never seen.
		missing := "bafkreigh2akiscaildcqabsyg3dfr6chu3fgpregiymsck7e7aqa4s52zy"
		res := nodes[0].RunIPFS("provide", "once", missing)
		assert.Contains(t, res.Stderr.Trimmed(), "not found locally, cannot provide")
		assert.Equal(t, 1, res.ExitCode())
	})

	t.Run("ipfs provide once --recursive announces every block in the DAG", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Enabled", true)
			// Selective strategy + --pin=false below means nothing is
			// auto-provided; everything findable comes from `provide once`.
			n.SetIPFSConfig("Provide.Strategy", "roots")
			// 1 MiB chunks so a 2 MiB file produces multiple leaf blocks.
			n.SetIPFSConfig("Import.UnixFSChunker", "size-1048576")
		})
		defer nodes.StopDaemons()

		publisher := nodes[0]
		data := random.Bytes(2 * 1024 * 1024)
		cidRoot := publisher.IPFSAdd(bytes.NewReader(data), "-Q", "--pin=false")

		// Discover a chunk CID via the root's DAG links.
		dagOut := publisher.IPFS("dag", "get", cidRoot)
		var dagNode struct {
			Links []struct {
				Hash map[string]string `json:"Hash"`
			} `json:"Links"`
		}
		require.NoError(t, json.Unmarshal(dagOut.Stdout.Bytes(), &dagNode))
		require.Greater(t, len(dagNode.Links), 1, "2 MiB file with 1 MiB chunker should have multiple chunks")
		cidChunk := dagNode.Links[0].Hash["/"]
		require.NotEmpty(t, cidChunk)

		// Recursive provide should announce both the root and every chunk.
		res := publisher.RunIPFS("provide", "once", "-r", cidRoot)
		assert.Equal(t, 0, res.ExitCode(), "provide once -r should succeed")
		expectProviders(t, cidRoot, publisher.PeerID().String(), nodes[1:]...)
		expectProviders(t, cidChunk, publisher.PeerID().String(), nodes[1:]...)
	})

	t.Run("ipfs provide once accepts multiple CIDs and reports count", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Enabled", true)
			n.SetIPFSConfig("Provide.Strategy", "roots")
		})
		defer nodes.StopDaemons()

		publisher := nodes[0]
		c1 := publisher.IPFSAddStr(uniq("multi 1"), "--pin=false")
		c2 := publisher.IPFSAddStr(uniq("multi 2"), "--pin=false")
		c3 := publisher.IPFSAddStr(uniq("multi 3"), "--pin=false")

		res := publisher.RunIPFS("provide", "once", c1, c2, c3)
		assert.Equal(t, 0, res.ExitCode(), "provide once with multiple CIDs should succeed")
		assert.Contains(t, res.Stdout.Trimmed(), "queued 3 CID(s) for immediate provide")

		expectProviders(t, c1, publisher.PeerID().String(), nodes[1:]...)
		expectProviders(t, c2, publisher.PeerID().String(), nodes[1:]...)
		expectProviders(t, c3, publisher.PeerID().String(), nodes[1:]...)
	})

	t.Run("ipfs provide once reads CIDs streamed from stdin", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Enabled", true)
			n.SetIPFSConfig("Provide.Strategy", "roots")
		})
		defer nodes.StopDaemons()

		publisher := nodes[0]
		c1 := publisher.IPFSAddStr(uniq("stdin 1"), "--pin=false")
		c2 := publisher.IPFSAddStr(uniq("stdin 2"), "--pin=false")
		c3 := publisher.IPFSAddStr(uniq("stdin 3"), "--pin=false")

		res := publisher.Runner.Run(harness.RunRequest{
			Path: publisher.IPFSBin,
			Args: []string{"provide", "once"},
			CmdOpts: []harness.CmdOpt{
				harness.RunWithStdinStr(c1 + "\n" + c2 + "\n" + c3 + "\n"),
			},
		})
		assert.Equal(t, 0, res.ExitCode(), "provide once with stdin should succeed")
		assert.Contains(t, res.Stdout.Trimmed(), "queued 3 CID(s) for immediate provide")

		expectProviders(t, c1, publisher.PeerID().String(), nodes[1:]...)
		expectProviders(t, c2, publisher.PeerID().String(), nodes[1:]...)
		expectProviders(t, c3, publisher.PeerID().String(), nodes[1:]...)
	})

	t.Run("ipfs provide once deduplicates repeated CIDs", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 1, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Enabled", true)
			n.SetIPFSConfig("Provide.Strategy", "roots")
		})
		defer nodes.StopDaemons()

		publisher := nodes[0]
		c1 := publisher.IPFSAddStr(uniq("dedup 1"), "--pin=false")
		c2 := publisher.IPFSAddStr(uniq("dedup 2"), "--pin=false")

		// 4 args, 2 unique CIDs. The repeated ones should not produce
		// extra events on the wire.
		res := publisher.RunIPFS("provide", "once", "--enc=json", c1, c2, c1, c2)
		assert.Equal(t, 0, res.ExitCode())

		var queued []string
		for line := range strings.Lines(res.Stdout.String()) {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var ev struct{ Queued string }
			require.NoError(t, json.Unmarshal([]byte(line), &ev))
			queued = append(queued, ev.Queued)
		}
		assert.ElementsMatch(t, []string{c1, c2}, queued, "duplicates should be filtered")
	})

	t.Run("ipfs provide once --enc=json streams one event per CID", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 1, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Enabled", true)
			n.SetIPFSConfig("Provide.Strategy", "roots")
		})
		defer nodes.StopDaemons()

		publisher := nodes[0]
		c1 := publisher.IPFSAddStr(uniq("json 1"), "--pin=false")
		c2 := publisher.IPFSAddStr(uniq("json 2"), "--pin=false")

		res := publisher.RunIPFS("provide", "once", "--enc=json", c1, c2)
		assert.Equal(t, 0, res.ExitCode(), "provide once --enc=json should succeed")

		// Parse one JSON object per non-empty line.
		var queued []string
		for line := range strings.Lines(res.Stdout.String()) {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var ev struct{ Queued string }
			require.NoError(t, json.Unmarshal([]byte(line), &ev), "each line must parse as JSON: %q", line)
			queued = append(queued, ev.Queued)
		}
		assert.ElementsMatch(t, []string{c1, c2}, queued)
	})

	t.Run("Provide.DHT.Interval=0 keeps announcing new CIDs (fast-provide-root)", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			// Required: Interval=0 alone is rejected by the validator
			// since the new semantic only disables the schedule.
			n.SetIPFSConfig("Provide.Enabled", true)
			n.SetIPFSConfig("Provide.DHT.Interval", "0")
		})
		defer nodes.StopDaemons()

		cid := nodes[0].IPFSAddStr(time.Now().String())
		expectProviders(t, cid, nodes[0].PeerID().String(), nodes[1:]...)
	})

	// `routing reprovide` is only available with the legacy provider.
	// Sweep provider reprovides automatically on schedule.
	if !sweep {
		t.Run("Manual Reprovide trigger does not work when periodic reprovide is disabled", func(t *testing.T) {
			t.Parallel()

			nodes := initNodes(t, 2, func(n *harness.Node) {
				n.SetIPFSConfig("Provide.Enabled", true)
				n.SetIPFSConfig("Provide.DHT.Interval", "0")
			})
			defer nodes.StopDaemons()

			res := nodes[0].RunIPFS("routing", "reprovide")
			assert.Contains(t, res.Stderr.Trimmed(), "invalid configuration: Provide.DHT.Interval is set to '0'")
			assert.Equal(t, 1, res.ExitCode())
		})

		t.Run("Manual Reprovide trigger does not work when Provide system is disabled", func(t *testing.T) {
			t.Parallel()

			nodes := initNodes(t, 2, func(n *harness.Node) {
				n.SetIPFSConfig("Provide.Enabled", false)
			})
			defer nodes.StopDaemons()

			cid := nodes[0].IPFSAddStr(time.Now().String())

			expectNoProviders(t, cid, nodes[1:]...)

			res := nodes[0].RunIPFS("routing", "reprovide")
			assert.Contains(t, res.Stderr.Trimmed(), "invalid configuration: Provide.Enabled is set to 'false'")
			assert.Equal(t, 1, res.ExitCode())

			expectNoProviders(t, cid, nodes[1:]...)
		})
	}

	t.Run("Provide with 'all' strategy", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Strategy", "all")
		})
		defer nodes.StopDaemons()
		publisher := nodes[0]

		cid := publisher.IPFSAddStr(uniq("all strategy"))
		expectProviders(t, cid, publisher.PeerID().String(), nodes[1:]...)
	})

	t.Run("Provide with 'pinned' strategy", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Strategy", "pinned")
		})
		defer nodes.StopDaemons()
		publisher := nodes[0]

		// Add a non-pinned CID (should not be provided)
		cid := publisher.IPFSAddStr(uniq("pinned strategy"), "--pin=false")
		expectNoProviders(t, cid, nodes[1:]...)

		// Pin the CID (should now be provided)
		publisher.IPFS("pin", "add", cid)
		expectProviders(t, cid, publisher.PeerID().String(), nodes[1:]...)
	})

	t.Run("Provide with 'pinned+mfs' strategy", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Strategy", "pinned+mfs")
		})
		defer nodes.StopDaemons()
		publisher := nodes[0]

		cidPinned := publisher.IPFSAddStr(uniq("pinned content"))
		cidUnpinned := publisher.IPFSAddStr(uniq("unpinned content"), "--pin=false")
		cidMFS := publisher.IPFSAddStr(uniq("mfs content"), "--pin=false")
		publisher.IPFS("files", "cp", "/ipfs/"+cidMFS, "/myfile")

		expectProviders(t, cidPinned, publisher.PeerID().String(), nodes[1:]...)
		expectNoProviders(t, cidUnpinned, nodes[1:]...)
		expectProviders(t, cidMFS, publisher.PeerID().String(), nodes[1:]...)
	})

	// addLargeFileInSubdir adds a 2 MiB file inside /subdir/ in MFS and
	// returns the MFS root CID, the file root CID, and a chunk CID.
	// The file is large enough to be split into multiple blocks.
	// The resulting DAG: root-dir/subdir/largefile (2+ chunks).
	addLargeFileInSubdir := func(t *testing.T, publisher *harness.Node) (cidRoot, cidSubdir, cidFile, cidChunk string) {
		t.Helper()
		largeData := random.Bytes(2 * 1024 * 1024) // 2 MiB = 2 chunks at 1 MiB

		// Add file without pinning, then build directory structure in MFS
		cidFile = publisher.IPFSAdd(bytes.NewReader(largeData), "-Q", "--pin=false")
		publisher.IPFS("files", "mkdir", "-p", "/subdir")
		publisher.IPFS("files", "cp", "/ipfs/"+cidFile, "/subdir/largefile")

		// Get CIDs for the directory structure
		cidRoot = publisher.IPFS("files", "stat", "--hash", "/").Stdout.Trimmed()
		cidSubdir = publisher.IPFS("files", "stat", "--hash", "/subdir").Stdout.Trimmed()

		// Get a chunk CID from the file's DAG links
		dagOut := publisher.IPFS("dag", "get", cidFile)
		var dagNode struct {
			Links []struct {
				Hash map[string]string `json:"Hash"`
			} `json:"Links"`
		}
		require.NoError(t, json.Unmarshal(dagOut.Stdout.Bytes(), &dagNode))
		require.Greater(t, len(dagNode.Links), 1, "file should have multiple chunks")
		cidChunk = dagNode.Links[0].Hash["/"]
		require.NotEmpty(t, cidChunk)

		return cidRoot, cidSubdir, cidFile, cidChunk
	}

	// +unique and +entities tests verify which CIDs end up in the DHT
	// (strategy scope). Bloom filter deduplication correctness and
	// entity type detection are tested in boxo/dag/walker/*_test.go.

	t.Run("Provide with 'pinned+mfs+unique' strategy", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Strategy", "pinned+mfs+unique")
			n.SetIPFSConfig("Import.UnixFSChunker", "size-1048576") // 1 MiB chunks
		})
		defer nodes.StopDaemons()
		publisher, peers := nodes[0], nodes[1:]

		// +unique provides all blocks in pinned DAGs (same scope as
		// pinned+mfs but with bloom filter dedup across pins).
		// Use --fast-provide-dag and --fast-provide-wait on pin add
		// so we can verify which blocks the strategy includes.
		cidRoot, cidSubdir, cidFile, cidChunk := addLargeFileInSubdir(t, publisher)
		publisher.IPFS("pin", "add", "--fast-provide-dag", "--fast-provide-wait", cidRoot)
		cidUnpinned := publisher.IPFSAddStr(uniq("unpinned content"), "--pin=false")

		pid := publisher.PeerID().String()
		// All blocks in the pinned DAG should be provided (including chunks)
		expectProviders(t, cidRoot, pid, peers...)
		expectProviders(t, cidSubdir, pid, peers...)
		expectProviders(t, cidFile, pid, peers...)
		expectProviders(t, cidChunk, pid, peers...)
		expectNoProviders(t, cidUnpinned, peers...)
	})

	t.Run("Provide with 'pinned+mfs+entities' strategy", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Strategy", "pinned+mfs+entities")
			n.SetIPFSConfig("Import.UnixFSChunker", "size-1048576") // 1 MiB chunks
		})
		defer nodes.StopDaemons()
		publisher, peers := nodes[0], nodes[1:]

		// +entities provides only entity roots (files, directories,
		// HAMT shards) and skips internal file chunks.
		// Use --fast-provide-dag and --fast-provide-wait on pin add
		// so we can verify which blocks the strategy skips.
		cidRoot, cidSubdir, cidFile, cidChunk := addLargeFileInSubdir(t, publisher)
		publisher.IPFS("pin", "add", "--fast-provide-dag", "--fast-provide-wait", cidRoot)

		pid := publisher.PeerID().String()
		// Entity roots: directories and file root
		expectProviders(t, cidRoot, pid, peers...)
		expectProviders(t, cidSubdir, pid, peers...)
		expectProviders(t, cidFile, pid, peers...)
		// Internal chunk should NOT be provided (+entities skips chunks)
		expectNoProviders(t, cidChunk, peers...)
	})

	t.Run("ipfs add --fast-provide-dag honors +entities (no chunk providing)", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Strategy", "pinned+entities")
			n.SetIPFSConfig("Import.UnixFSChunker", "size-1048576") // 1 MiB chunks
		})
		defer nodes.StopDaemons()
		publisher, peers := nodes[0], nodes[1:]

		// Regression test for the providingDagService double-providing
		// path. Before the fix, ipfs add --pin --fast-provide-dag wrapped
		// the DAGService with providingDagService, which announced every
		// block as it was written -- including chunks -- regardless of
		// the +entities modifier. The post-add ExecuteFastProvideDAG
		// walk then ran in parallel, so chunks ended up in the DHT
		// despite +entities saying they should be skipped.
		//
		// After the fix, ExecuteFastProvideDAG is the single mechanism
		// for --fast-provide-dag and respects the active strategy.
		largeData := random.Bytes(2 * 1024 * 1024) // 2 MiB = 2 chunks
		cidFile := publisher.IPFSAdd(bytes.NewReader(largeData),
			"--fast-provide-dag", "--fast-provide-wait")

		// Get a chunk CID from the file's DAG links
		dagOut := publisher.IPFS("dag", "get", cidFile)
		var dagNode struct {
			Links []struct {
				Hash map[string]string `json:"Hash"`
			} `json:"Links"`
		}
		require.NoError(t, json.Unmarshal(dagOut.Stdout.Bytes(), &dagNode))
		require.Greater(t, len(dagNode.Links), 1, "file should have multiple chunks")
		cidChunk := dagNode.Links[0].Hash["/"]
		require.NotEmpty(t, cidChunk)

		pid := publisher.PeerID().String()
		// File root (entity) should be provided
		expectProviders(t, cidFile, pid, peers...)
		// Chunk should NOT be provided (+entities skips chunks)
		expectNoProviders(t, cidChunk, peers...)
	})

	// addLargeFilestoreFile writes a 2 MiB file to the publisher's
	// node directory and adds it via --nocopy, returning the root CID
	// and a chunk CID from the file's DAG links. With the configured
	// 1 MiB chunker the file produces multiple leaf blocks so we can
	// distinguish root-level from chunk-level provide behavior.
	addLargeFilestoreFile := func(t *testing.T, publisher *harness.Node, addArgs ...string) (cidRoot, cidChunk string) {
		t.Helper()
		filePath := filepath.Join(publisher.Dir, "filestore-"+strconv.FormatInt(time.Now().UnixNano(), 10)+".bin")
		require.NoError(t, os.WriteFile(filePath, random.Bytes(2*1024*1024), 0o644))

		args := append([]string{"add", "-q", "--nocopy"}, addArgs...)
		args = append(args, filePath)
		cidRoot = strings.TrimSpace(publisher.IPFS(args...).Stdout.String())

		dagOut := publisher.IPFS("dag", "get", cidRoot)
		var dagNode struct {
			Links []struct {
				Hash map[string]string `json:"Hash"`
			} `json:"Links"`
		}
		require.NoError(t, json.Unmarshal(dagOut.Stdout.Bytes(), &dagNode))
		require.Greater(t, len(dagNode.Links), 1, "filestore file should have multiple chunks")
		cidChunk = dagNode.Links[0].Hash["/"]
		require.NotEmpty(t, cidChunk)
		return cidRoot, cidChunk
	}

	t.Run("Filestore --nocopy with 'all' strategy provides every block", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Experimental.FilestoreEnabled", true)
			n.SetIPFSConfig("Provide.Strategy", "all")
			n.SetIPFSConfig("Import.UnixFSChunker", "size-1048576") // 1 MiB chunks
		})
		defer nodes.StopDaemons()
		publisher, peers := nodes[0], nodes[1:]

		// Positive control: with the default 'all' strategy the
		// filestore Put path provides every block as it is written,
		// including non-root chunks.
		cidRoot, cidChunk := addLargeFilestoreFile(t, publisher)

		pid := publisher.PeerID().String()
		expectProviders(t, cidRoot, pid, peers...)
		expectProviders(t, cidChunk, pid, peers...)
	})

	t.Run("Filestore --nocopy with selective strategy skips write-time provide", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Experimental.FilestoreEnabled", true)
			n.SetIPFSConfig("Provide.Strategy", "pinned")
			n.SetIPFSConfig("Import.UnixFSChunker", "size-1048576") // 1 MiB chunks
		})
		defer nodes.StopDaemons()
		publisher, peers := nodes[0], nodes[1:]

		// With a selective strategy the filestore must not eagerly
		// announce blocks at write time. --pin=false skips the pin
		// (so fast-provide-root has nothing to do) and
		// --fast-provide-root=false disables it explicitly, isolating
		// the assertion to the filestore's internal provide path.
		cidRoot, cidChunk := addLargeFilestoreFile(t, publisher,
			"--pin=false", "--fast-provide-root=false")

		expectNoProviders(t, cidRoot, peers...)
		expectNoProviders(t, cidChunk, peers...)
	})

	t.Run("Filestore --nocopy + selective strategy + --fast-provide-dag walks DAG", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Experimental.FilestoreEnabled", true)
			n.SetIPFSConfig("Provide.Strategy", "pinned")
			n.SetIPFSConfig("Import.UnixFSChunker", "size-1048576") // 1 MiB chunks
		})
		defer nodes.StopDaemons()
		publisher, peers := nodes[0], nodes[1:]

		// The selective-strategy gate skips the filestore's write-time
		// provide, but the post-add ExecuteFastProvideDAG walk reads
		// blocks through the wrapping blockstore (which transparently
		// serves filestore-backed content) and announces each block,
		// honoring the active strategy. This is the integration test
		// behind the changelog claim that filestore content now plays
		// well with the fast-provide-dag flag.
		cidRoot, cidChunk := addLargeFilestoreFile(t, publisher,
			"--fast-provide-dag", "--fast-provide-wait")

		pid := publisher.PeerID().String()
		expectProviders(t, cidRoot, pid, peers...)
		expectProviders(t, cidChunk, pid, peers...)
	})

	t.Run("Provide with 'roots' strategy", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Strategy", "roots")
		})
		defer nodes.StopDaemons()
		publisher := nodes[0]

		// Add with -w: the wrapper directory is the recursive pin root,
		// the file inside is a child block of that pin (not a root).
		// Use --only-hash first to learn the child CID without providing.
		data := random.Bytes(1000)
		cidChild := publisher.IPFSAdd(bytes.NewReader(data), "-Q", "--only-hash")
		cidRoot := publisher.IPFSAdd(bytes.NewReader(data), "-Q", "-w")

		// 'roots' strategy provides only pin roots, not child blocks.
		expectProviders(t, cidRoot, publisher.PeerID().String(), nodes[1:]...)
		expectNoProviders(t, cidChild, nodes[1:]...)
	})

	t.Run("Provide with 'mfs' strategy", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Strategy", "mfs")
		})
		defer nodes.StopDaemons()
		publisher := nodes[0]

		// 'mfs' only provides content in MFS. Pinned content outside
		// MFS should NOT be provided (mfs excludes pinned by default).
		cidPinned := publisher.IPFSAddStr(uniq("pinned but not mfs"))
		expectNoProviders(t, cidPinned, nodes[1:]...)

		// Add to MFS (should be provided)
		data := random.Bytes(1000)
		cidMFS := publisher.IPFSAdd(bytes.NewReader(data), "-Q", "--pin=false")
		publisher.IPFS("files", "cp", "/ipfs/"+cidMFS, "/myfile")
		expectProviders(t, cidMFS, publisher.PeerID().String(), nodes[1:]...)

		// Pinned CID still not provided (mfs strategy ignores pins)
		expectNoProviders(t, cidPinned, nodes[1:]...)
	})

	// Reprovide tests: add content offline, start daemon, wait for reprovide.
	//
	// Each test waits for TWO reprovide cycles to confirm the schedule
	// works repeatedly, not just on the initial bootstrap. The second
	// cycle also catches bugs where state isn't persisted across cycles.
	//
	// Legacy: `routing reprovide` blocks until the reprovide cycle finishes,
	// so we call it and check results immediately after.
	//
	// Sweep: no manual trigger exists. Instead, we set
	// Provide.DHT.Interval=30s on the importing node and poll
	// `provide stat` until the cycle completes.

	// verifyReprovide waits for two reprovide cycles and asserts which
	// CIDs are/aren't findable after each. minCIDs is the expected
	// number of provided CIDs per cycle.
	verifyReprovide := func(
		t *testing.T,
		publisher *harness.Node,
		queriers harness.Nodes,
		minCIDs int64,
		provided []string,
		notProvided []string,
	) {
		t.Helper()
		pid := publisher.PeerID().String()
		check := func() {
			for _, c := range provided {
				expectProviders(t, c, pid, queriers...)
			}
			for _, c := range notProvided {
				expectNoProviders(t, c, queriers...)
			}
		}

		after1 := awaitReprovide(t, publisher, minCIDs)
		check()
		// Second cycle: confirms the schedule runs repeatedly.
		awaitReprovide(t, publisher, after1+minCIDs)
		check()
	}

	{

		t.Run("Reprovides with 'all' strategy when strategy is '' (empty)", func(t *testing.T) {
			t.Parallel()

			nodes := initNodesWithoutStart(t, 2, func(n *harness.Node) {
				n.SetIPFSConfig("Provide.Strategy", "")
			})
			publisher := nodes[0]
			if sweep {
				publisher.SetIPFSConfig("Provide.DHT.Interval", "30s")
			}

			cid := publisher.IPFSAddStr(time.Now().String())

			nodes = nodes.StartDaemons().Connect()
			defer nodes.StopDaemons()
			peers := nodes[1:]

			verifyReprovide(t, publisher, peers, 1, // 1 block added
				[]string{cid}, nil)
		})

		t.Run("Reprovides with 'all' strategy", func(t *testing.T) {
			t.Parallel()

			nodes := initNodesWithoutStart(t, 2, func(n *harness.Node) {
				n.SetIPFSConfig("Provide.Strategy", "all")
			})
			publisher := nodes[0]
			if sweep {
				publisher.SetIPFSConfig("Provide.DHT.Interval", "30s")
			}

			cid := publisher.IPFSAddStr(time.Now().String())

			nodes = nodes.StartDaemons().Connect()
			defer nodes.StopDaemons()
			peers := nodes[1:]

			verifyReprovide(t, publisher, peers, 1, // 1 block added
				[]string{cid}, nil)
		})

		t.Run("Reprovides with 'pinned' strategy", func(t *testing.T) {
			t.Parallel()

			foo := random.Bytes(1000)
			bar := random.Bytes(1000)

			nodes := initNodesWithoutStart(t, 2, func(n *harness.Node) {
				n.SetIPFSConfig("Provide.Strategy", "pinned")
			})
			publisher := nodes[0]
			if sweep {
				publisher.SetIPFSConfig("Provide.DHT.Interval", "30s")
			}

			// Add a pin while offline
			cidBarDir := publisher.IPFSAdd(bytes.NewReader(bar), "-Q", "-w")

			nodes = nodes.StartDaemons().Connect()
			defer nodes.StopDaemons()
			peers := nodes[1:]

			// Add content without pinning while daemon is online
			cidFoo := publisher.IPFSAdd(bytes.NewReader(foo), "--pin=false")
			cidBar := publisher.IPFSAdd(bytes.NewReader(bar), "--pin=false")

			verifyReprovide(t, publisher, peers, 2, // cidBar + cidBarDir (bar is child of the wrapped dir pin)
				[]string{cidBar, cidBarDir},
				[]string{cidFoo}) // cidFoo not pinned
		})

		t.Run("Reprovides with 'roots' strategy", func(t *testing.T) {
			t.Parallel()

			bar := random.Bytes(1000)

			nodes := initNodesWithoutStart(t, 2, func(n *harness.Node) {
				n.SetIPFSConfig("Provide.Strategy", "roots")
			})
			publisher := nodes[0]
			if sweep {
				publisher.SetIPFSConfig("Provide.DHT.Interval", "30s")
			}

			// Compute the child CID without storing anything (safe
			// offline, daemon not started yet).
			cidChild := publisher.IPFSAdd(bytes.NewReader(bar), "-Q", "--only-hash")
			// Add with -w: pins the wrapper directory as root. The file
			// inside is a child block of that pin, not a root.
			cidRoot := publisher.IPFSAdd(bytes.NewReader(bar), "-Q", "-w")

			nodes = nodes.StartDaemons().Connect()
			defer nodes.StopDaemons()
			peers := nodes[1:]

			verifyReprovide(t, publisher, peers, 1, // cidRoot (only pin root)
				[]string{cidRoot},
				[]string{cidChild}) // child of pin, not a root
		})

		t.Run("Reprovides with 'mfs' strategy", func(t *testing.T) {
			t.Parallel()

			bar := random.Bytes(1000)

			nodes := initNodesWithoutStart(t, 2, func(n *harness.Node) {
				n.SetIPFSConfig("Provide.Strategy", "mfs")
			})
			publisher := nodes[0]
			if sweep {
				publisher.SetIPFSConfig("Provide.DHT.Interval", "30s")
			}

			// Add to MFS (should be provided)
			cidMFS := publisher.IPFSAdd(bytes.NewReader(bar), "--pin=false", "-Q")
			publisher.IPFS("files", "cp", "/ipfs/"+cidMFS, "/myfile")
			// Pin something NOT in MFS (should NOT be provided)
			cidPinned := publisher.IPFSAddStr(uniq("pinned but not mfs"))

			nodes = nodes.StartDaemons().Connect()
			defer nodes.StopDaemons()
			peers := nodes[1:]

			verifyReprovide(t, publisher, peers, 1, // cidMFS only
				[]string{cidMFS},
				[]string{cidPinned}) // mfs strategy ignores pinned content outside MFS
		})

		t.Run("Reprovides with 'pinned+mfs' strategy", func(t *testing.T) {
			t.Parallel()

			nodes := initNodesWithoutStart(t, 2, func(n *harness.Node) {
				n.SetIPFSConfig("Provide.Strategy", "pinned+mfs")
			})
			publisher := nodes[0]
			if sweep {
				publisher.SetIPFSConfig("Provide.DHT.Interval", "30s")
			}

			// Add a pinned CID (should be provided)
			cidPinned := publisher.IPFSAddStr(uniq("pinned content"), "--pin=true")
			// Add a CID to MFS (should be provided)
			cidMFS := publisher.IPFSAddStr(uniq("mfs content"))
			publisher.IPFS("files", "cp", "/ipfs/"+cidMFS, "/myfile")
			// Add a CID that is neither pinned nor in MFS (should not be provided)
			cidNeither := publisher.IPFSAddStr(uniq("neither content"), "--pin=false")

			nodes = nodes.StartDaemons().Connect()
			defer nodes.StopDaemons()
			peers := nodes[1:]

			verifyReprovide(t, publisher, peers, 2, // cidPinned + cidMFS
				[]string{cidPinned, cidMFS},
				[]string{cidNeither}) // neither pinned nor in MFS
		})

		t.Run("Reprovides with 'pinned+mfs+unique' strategy", func(t *testing.T) {
			t.Parallel()

			nodes := initNodesWithoutStart(t, 2, func(n *harness.Node) {
				n.SetIPFSConfig("Provide.Strategy", "pinned+mfs+unique")
				n.SetIPFSConfig("Import.UnixFSChunker", "size-1048576") // 1 MiB chunks
			})
			publisher := nodes[0]
			if sweep {
				publisher.SetIPFSConfig("Provide.DHT.Interval", "30s")
			}

			// Build a directory DAG with a multi-chunk file in MFS, then pin it.
			cidRoot, cidSubdir, cidFile, cidChunk := addLargeFileInSubdir(t, publisher)
			publisher.IPFS("pin", "add", cidRoot)
			cidUnpinned := publisher.IPFSAddStr(uniq("unpinned content"), "--pin=false")

			nodes = nodes.StartDaemons().Connect()
			defer nodes.StopDaemons()
			peers := nodes[1:]

			// +unique provides all blocks in pinned DAGs (same as pinned+mfs)
			verifyReprovide(t, publisher, peers, 4, // root + subdir + file + chunks
				[]string{cidRoot, cidSubdir, cidFile, cidChunk},
				[]string{cidUnpinned})
		})

		t.Run("Reprovides with 'pinned+mfs+entities' strategy", func(t *testing.T) {
			t.Parallel()

			nodes := initNodesWithoutStart(t, 2, func(n *harness.Node) {
				n.SetIPFSConfig("Provide.Strategy", "pinned+mfs+entities")
				n.SetIPFSConfig("Import.UnixFSChunker", "size-1048576") // 1 MiB chunks
			})
			publisher := nodes[0]
			if sweep {
				publisher.SetIPFSConfig("Provide.DHT.Interval", "30s")
			}

			// Build a directory DAG with a multi-chunk file in MFS, then pin it.
			cidRoot, cidSubdir, cidFile, cidChunk := addLargeFileInSubdir(t, publisher)
			publisher.IPFS("pin", "add", cidRoot)

			nodes = nodes.StartDaemons().Connect()
			defer nodes.StopDaemons()
			peers := nodes[1:]

			// Entity roots: directories and file root (not chunks)
			verifyReprovide(t, publisher, peers, 3, // root + subdir + file (not chunks)
				[]string{cidRoot, cidSubdir, cidFile},
				[]string{cidChunk}) // chunks skipped by +entities
		})
	}

	t.Run("provide clear command removes items from provide queue", func(t *testing.T) {
		t.Parallel()

		nodes := harness.NewT(t).NewNodes(1).Init()
		nodes.ForEachPar(func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Enabled", true)
			n.SetIPFSConfig("Provide.DHT.Interval", "22h")
			n.SetIPFSConfig("Provide.Strategy", "all")
		})
		nodes.StartDaemons()
		defer nodes.StopDaemons()

		// Clear the provide queue first time - works regardless of queue state
		res1 := nodes[0].IPFS("provide", "clear")
		require.NoError(t, res1.Err)

		// Should report cleared items and proper message format
		assert.Contains(t, res1.Stdout.String(), "removed")
		assert.Contains(t, res1.Stdout.String(), "items from provide queue")

		// Clear the provide queue second time - should definitely report 0 items
		res2 := nodes[0].IPFS("provide", "clear")
		require.NoError(t, res2.Err)

		// Should report 0 items cleared since queue was already cleared
		assert.Contains(t, res2.Stdout.String(), "removed 0 items from provide queue")
	})

	t.Run("provide clear command with quiet option", func(t *testing.T) {
		t.Parallel()

		nodes := harness.NewT(t).NewNodes(1).Init()
		nodes.ForEachPar(func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Enabled", true)
			n.SetIPFSConfig("Provide.DHT.Interval", "22h")
			n.SetIPFSConfig("Provide.Strategy", "all")
		})
		nodes.StartDaemons()
		defer nodes.StopDaemons()

		// Clear the provide queue with quiet option
		res := nodes[0].IPFS("provide", "clear", "-q")
		require.NoError(t, res.Err)

		// Should have no output when quiet
		assert.Empty(t, res.Stdout.String())
	})

	t.Run("provide clear command works when provider is disabled", func(t *testing.T) {
		t.Parallel()

		nodes := harness.NewT(t).NewNodes(1).Init()
		nodes.ForEachPar(func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Enabled", false)
			n.SetIPFSConfig("Provide.DHT.Interval", "22h")
			n.SetIPFSConfig("Provide.Strategy", "all")
		})
		nodes.StartDaemons()
		defer nodes.StopDaemons()

		// Clear should succeed even when provider is disabled
		res := nodes[0].IPFS("provide", "clear")
		require.NoError(t, res.Err)
	})

	t.Run("provide clear command returns JSON with removed item count", func(t *testing.T) {
		t.Parallel()

		nodes := harness.NewT(t).NewNodes(1).Init()
		nodes.ForEachPar(func(n *harness.Node) {
			n.SetIPFSConfig("Provide.Enabled", true)
			n.SetIPFSConfig("Provide.DHT.Interval", "22h")
			n.SetIPFSConfig("Provide.Strategy", "all")
		})
		nodes.StartDaemons()
		defer nodes.StopDaemons()

		// Clear the provide queue with JSON encoding
		res := nodes[0].IPFS("provide", "clear", "--enc=json")
		require.NoError(t, res.Err)

		// Should return valid JSON with the number of removed items
		output := res.Stdout.String()
		assert.NotEmpty(t, output)

		// Parse JSON to verify structure
		var result int
		err := json.Unmarshal([]byte(output), &result)
		require.NoError(t, err, "Output should be valid JSON")

		// Should be a non-negative integer (0 or positive)
		assert.GreaterOrEqual(t, result, 0)
	})
}

// runResumeTests validates Provide.DHT.ResumeEnabled behavior for SweepingProvider.
//
// Background: The provider tracks current_time_offset = (now - cycleStart) % interval
// where cycleStart is the timestamp marking the beginning of the reprovide cycle.
// With ResumeEnabled=true, cycleStart persists in the datastore across restarts.
// With ResumeEnabled=false, cycleStart resets to 'now' on each startup.
func runResumeTests(t *testing.T, apply cfgApplier) {
	t.Helper()

	const (
		reprovideInterval = 30 * time.Second
		initialRuntime    = 10 * time.Second // Let cycle progress
		downtime          = 5 * time.Second  // Simulated offline period
		restartTime       = 2 * time.Second  // Daemon restart stabilization

		// Thresholds account for timing jitter (~2-3s margin)
		minOffsetBeforeRestart = 8 * time.Second  // Expect ~10s
		minOffsetAfterResume   = 12 * time.Second // Expect ~17s (10s + 5s + 2s)
		maxOffsetAfterReset    = 5 * time.Second  // Expect ~2s (fresh start)
	)

	setupNode := func(t *testing.T, resumeEnabled bool) *harness.Node {
		node := harness.NewT(t).NewNode().Init()
		apply(node) // Sets Provide.DHT.SweepEnabled=true
		node.SetIPFSConfig("Provide.DHT.ResumeEnabled", resumeEnabled)
		node.SetIPFSConfig("Provide.DHT.Interval", reprovideInterval.String())
		node.SetIPFSConfig("Bootstrap", []string{})
		node.StartDaemon()
		return node
	}

	t.Run("preserves cycle state across restart", func(t *testing.T) {
		t.Parallel()

		node := setupNode(t, true)
		defer node.StopDaemon()

		for i := range 10 {
			node.IPFSAddStr(fmt.Sprintf("resume-test-%d-%d", i, time.Now().UnixNano()))
		}

		time.Sleep(initialRuntime)

		beforeRestart := node.IPFS("provide", "stat", "--enc=json")
		offsetBeforeRestart, _, err := parseProvideStatJSON(beforeRestart.Stdout.String())
		require.NoError(t, err)
		require.Greater(t, offsetBeforeRestart, minOffsetBeforeRestart,
			"cycle should have progressed")

		node.StopDaemon()
		time.Sleep(downtime)
		node.StartDaemon()
		time.Sleep(restartTime)

		afterRestart := node.IPFS("provide", "stat", "--enc=json")
		offsetAfterRestart, _, err := parseProvideStatJSON(afterRestart.Stdout.String())
		require.NoError(t, err)

		assert.GreaterOrEqual(t, offsetAfterRestart, minOffsetAfterResume,
			"offset should account for downtime")
	})

	t.Run("resets cycle when disabled", func(t *testing.T) {
		t.Parallel()

		node := setupNode(t, false)
		defer node.StopDaemon()

		for i := range 10 {
			node.IPFSAddStr(fmt.Sprintf("no-resume-%d-%d", i, time.Now().UnixNano()))
		}

		time.Sleep(initialRuntime)

		beforeRestart := node.IPFS("provide", "stat", "--enc=json")
		offsetBeforeRestart, _, err := parseProvideStatJSON(beforeRestart.Stdout.String())
		require.NoError(t, err)
		require.Greater(t, offsetBeforeRestart, minOffsetBeforeRestart,
			"cycle should have progressed")

		node.StopDaemon()
		time.Sleep(downtime)
		node.StartDaemon()
		time.Sleep(restartTime)

		afterRestart := node.IPFS("provide", "stat", "--enc=json")
		offsetAfterRestart, _, err := parseProvideStatJSON(afterRestart.Stdout.String())
		require.NoError(t, err)

		assert.Less(t, offsetAfterRestart, maxOffsetAfterReset,
			"offset should reset to near zero")
	})
}

type provideStatJSON struct {
	Sweep struct {
		Timing struct {
			CurrentTimeOffset int64 `json:"current_time_offset"` // nanoseconds
		} `json:"timing"`
		Schedule struct {
			NextReprovidePrefix string `json:"next_reprovide_prefix"`
		} `json:"schedule"`
		Operations struct {
			Ongoing struct {
				KeyReprovides int `json:"key_reprovides"`
			} `json:"ongoing"`
			Past struct {
				KeysProvided int64 `json:"keys_provided"`
			} `json:"past"`
		} `json:"operations"`
		Queues struct {
			PendingKeyProvides int64 `json:"pending_key_provides"`
		} `json:"queues"`
	} `json:"Sweep"`
}

// parseProvideStatJSON extracts timing and schedule information from
// the JSON output of 'ipfs provide stat --enc=json'.
func parseProvideStatJSON(output string) (offset time.Duration, prefix string, err error) {
	var stat provideStatJSON
	if err := json.Unmarshal([]byte(output), &stat); err != nil {
		return 0, "", err
	}
	offset = time.Duration(stat.Sweep.Timing.CurrentTimeOffset)
	prefix = stat.Sweep.Schedule.NextReprovidePrefix
	return offset, prefix, nil
}

// waitForSweepReprovide polls `provide stat --enc=json` until the
// sweep provider has provided at least minCIDs and no work is pending.
// Pass 0 for minCIDs to just wait for any provide activity to finish.
// Returns the total CIDs provided so far (for use as minCIDs in a
// subsequent call to wait for the next cycle).
// The importing node must have a short Provide.DHT.Interval so the
// reprovide cycle completes within the timeout.
func waitForSweepReprovide(t *testing.T, n *harness.Node, timeout time.Duration, minCIDs int64) int64 {
	t.Helper()
	if minCIDs == 0 {
		minCIDs = 1
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		res := n.RunIPFS("provide", "stat", "--enc=json")
		if res.ExitCode() == 0 {
			var stat provideStatJSON
			if err := json.Unmarshal(res.Stdout.Bytes(), &stat); err == nil {
				s := stat.Sweep
				if s.Operations.Past.KeysProvided >= minCIDs &&
					s.Queues.PendingKeyProvides == 0 &&
					s.Operations.Ongoing.KeyReprovides == 0 {
					return s.Operations.Past.KeysProvided
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("sweep reprovide: expected at least %d CIDs provided within %s", minCIDs, timeout)
	return 0
}

func TestProvider(t *testing.T) {
	t.Parallel()

	variants := []struct {
		name           string
		sweep          bool
		apply          cfgApplier
		awaitReprovide awaitReprovideFunc
	}{
		{
			name:  "LegacyProvider",
			sweep: false,
			apply: func(n *harness.Node) {
				n.SetIPFSConfig("Provide.DHT.SweepEnabled", false)
			},
			// `routing reprovide` blocks until the cycle finishes.
			// minCIDs is ignored (legacy has no stat counter).
			awaitReprovide: func(t *testing.T, n *harness.Node, minCIDs int64) int64 {
				n.IPFS("routing", "reprovide")
				return minCIDs
			},
		},
		{
			name:  "SweepingProvider",
			sweep: true,
			apply: func(n *harness.Node) {
				n.SetIPFSConfig("Provide.DHT.SweepEnabled", true)
			},
			// No manual trigger exists for sweep. Poll `provide stat`
			// until the reprovide cycle completes.
			awaitReprovide: func(t *testing.T, n *harness.Node, minCIDs int64) int64 {
				// 90s accounts for provider bootstrap time (connecting
				// to ephemeral peers, measuring prefix length) before
				// the 30s reprovide cycle starts. On CI with parallel
				// tests, bootstrap can take 20-30s.
				return waitForSweepReprovide(t, n, 90*time.Second, minCIDs)
			},
		},
	}

	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			// t.Parallel()
			runProviderSuite(t, v.sweep, v.apply, v.awaitReprovide)

			// Resume tests only apply to SweepingProvider
			if v.sweep {
				runResumeTests(t, v.apply)
			}
		})
	}
}

// TestProviderUniqueDedupLogging verifies that the +unique bloom filter
// deduplication produces a "skippedBranches" log with a value > 0 when
// two pins share content. Tests both the fast-provide-dag path (immediate
// provide on pin add) and the reprovide cycle path.
func TestProviderUniqueDedupLogging(t *testing.T) {
	t.Parallel()

	// Shared data that both pins will reference. Two pins containing
	// the same file block give the bloom something to dedup.
	sharedData := random.Bytes(10 * 1024) // 10 KiB, single block

	t.Run("fast-provide-dag dedup across pins in single call", func(t *testing.T) {
		t.Parallel()

		h := harness.NewT(t)
		node := h.NewNode().Init()
		node.SetIPFSConfig("Provide.Strategy", "pinned+unique")
		node.SetIPFSConfig("Provide.DHT.SweepEnabled", true)
		node.SetIPFSConfig("Import.UnixFSChunker", "size-5120") // 5 KiB chunks
		h.BootstrapWithStubDHT(harness.Nodes{node})

		node.StartDaemonWithReq(harness.RunRequest{
			CmdOpts: []harness.CmdOpt{
				harness.RunWithEnv(map[string]string{
					// dagwalker: bloom creation log
					// core/commands/cmdenv: fast-provide-dag finished log
					"GOLOG_LOG_LEVEL": "error,dagwalker=info,core/commands/cmdenv=info",
				}),
			},
		}, "")
		defer node.StopDaemon()

		// 10 KiB file with 5 KiB chunks = 1 file root + 2 chunks = 3 blocks.
		// Two dirs each containing the file under different names:
		//   dirA/fileA → same 3 blocks
		//   dirB/fileB → same 3 blocks
		// Pinning both in a single `pin add` shares one bloom tracker.
		// Walking dirA: dirA + file root + chunk1 + chunk2 = 4 provided.
		// Walking dirB: dirB + file root (bloom hit, skip subtree) = 1 provided, 1 skipped.
		// Total: 5 provided, 1 skipped branch (file root in dirB; its
		// 2 chunks are never visited because the parent was skipped).
		cidFile := node.IPFSAdd(bytes.NewReader(sharedData), "-Q", "--pin=false")
		node.IPFS("files", "mkdir", "-p", "/dirA")
		node.IPFS("files", "cp", "/ipfs/"+cidFile, "/dirA/fileA")
		cidDirA := node.IPFS("files", "stat", "--hash", "/dirA").Stdout.Trimmed()
		node.IPFS("files", "mkdir", "-p", "/dirB")
		node.IPFS("files", "cp", "/ipfs/"+cidFile, "/dirB/fileB")
		cidDirB := node.IPFS("files", "stat", "--hash", "/dirB").Stdout.Trimmed()
		require.NotEqual(t, cidDirA, cidDirB, "dirs must differ to test dedup")
		// Single pin add with both CIDs shares one bloom.
		node.IPFS("pin", "add", "--fast-provide-dag", "--fast-provide-wait", cidDirA, cidDirB)

		daemonLog := node.Daemon.Stderr.String()
		require.Contains(t, daemonLog, "bloom tracker created")
		require.NotContains(t, daemonLog, "bloom tracker autoscaled")
		require.Contains(t, daemonLog, `"providedCIDs": 5`)
		require.Contains(t, daemonLog, `"skippedBranches": 1`)
	})

	t.Run("reprovide cycle dedup across pins", func(t *testing.T) {
		t.Parallel()

		h := harness.NewT(t)
		nodes := h.NewNodes(2).Init()
		for _, n := range nodes {
			n.SetIPFSConfig("Provide.Strategy", "pinned+unique")
			n.SetIPFSConfig("Provide.DHT.SweepEnabled", true)
			n.SetIPFSConfig("Import.UnixFSChunker", "size-5120") // 5 KiB chunks
		}
		publisher := nodes[0]
		publisher.SetIPFSConfig("Provide.DHT.Interval", "30s")
		h.BootstrapWithStubDHT(nodes)

		// Same file structure as fast-provide-dag test above.
		// The reprovide cycle walks all recursive pins:
		//   pin dirA: dirA + file root + chunk1 + chunk2 = 4 provided
		//   pin empty MFS root (always present): 1 provided
		//   pin dirB: dirB + file root (bloom hit, skip subtree) = 1 provided, 1 skipped
		// Total: 6 provided, 1 skipped branch.
		cidFile := publisher.IPFSAdd(bytes.NewReader(sharedData), "-Q", "--pin=false")
		publisher.IPFS("files", "mkdir", "-p", "/dirA")
		publisher.IPFS("files", "cp", "/ipfs/"+cidFile, "/dirA/fileA")
		cidDirA := publisher.IPFS("files", "stat", "--hash", "/dirA").Stdout.Trimmed()
		publisher.IPFS("pin", "add", cidDirA)
		publisher.IPFS("files", "mkdir", "-p", "/dirB")
		publisher.IPFS("files", "cp", "/ipfs/"+cidFile, "/dirB/fileB")
		cidDirB := publisher.IPFS("files", "stat", "--hash", "/dirB").Stdout.Trimmed()
		require.NotEqual(t, cidDirA, cidDirB, "dirs must differ to test dedup")
		publisher.IPFS("pin", "add", cidDirB)

		nodes[0].StartDaemonWithReq(harness.RunRequest{
			CmdOpts: []harness.CmdOpt{
				harness.RunWithEnv(map[string]string{
					"GOLOG_LOG_LEVEL": "error,dagwalker=info,provider=info",
				}),
			},
		}, "")
		nodes[1].StartDaemon()
		defer nodes.StopDaemons()
		nodes.Connect()

		waitForSweepReprovide(t, publisher, 90*time.Second, 6)

		daemonLog := publisher.Daemon.Stderr.String()
		require.Contains(t, daemonLog, "bloom tracker created")
		require.NotContains(t, daemonLog, "bloom tracker autoscaled")
		require.Contains(t, daemonLog, `"providedCIDs": 6`)
		require.Contains(t, daemonLog, `"skippedBranches": 1`)
	})
}

// TestProviderFastProvideDAGAsyncSurvives verifies that
// --fast-provide-dag without --fast-provide-wait runs a background
// DAG walk that outlives the command handler and publishes every
// block of the newly added DAG to the routing system.
//
// The async walk runs in a goroutine parented on the IpfsNode
// lifetime context (not req.Context), so it keeps running after
// `ipfs add` returns and is only cancelled on daemon shutdown.
//
// Provide.DHT.Interval is set high so the scheduled reprovide
// cycle cannot fire during the test window. That makes the async
// walk the only path that can publish non-root block CIDs.
func TestProviderFastProvideDAGAsyncSurvives(t *testing.T) {
	t.Parallel()

	h := harness.NewT(t)
	nodes := h.NewNodes(2).Init()
	for _, n := range nodes {
		n.SetIPFSConfig("Provide.Strategy", "pinned")
		n.SetIPFSConfig("Provide.DHT.SweepEnabled", true)
		// Small chunks so a modest file produces many leaf blocks.
		n.SetIPFSConfig("Import.UnixFSChunker", "size-1024")
	}
	publisher, peers := nodes[0], nodes[1:]
	publisher.SetIPFSConfig("Provide.DHT.Interval", "1h")
	h.BootstrapWithStubDHT(nodes)

	publisher.StartDaemonWithReq(harness.RunRequest{
		CmdOpts: []harness.CmdOpt{
			harness.RunWithEnv(map[string]string{
				"GOLOG_LOG_LEVEL": "error,core/commands/cmdenv=info",
			}),
		},
	}, "")
	nodes[1].StartDaemon()
	defer nodes.StopDaemons()
	nodes.Connect()

	// 16 KiB + 1 KiB chunks yields a file root plus many leaf
	// blocks, so the providedCIDs count after the walk is
	// unambiguous.
	data := random.Bytes(16 * 1024)
	cidFile := publisher.IPFSAdd(bytes.NewReader(data), "-Q",
		"--pin=true",
		"--fast-provide-dag=true",
		// --fast-provide-wait deliberately omitted: the walk
		// runs in the background after `ipfs add` returns.
	)

	// Pull a chunk CID out of the file DAG. Chunks are not pin
	// roots, so fast-provide-root does not touch them; only the
	// DAG walk can announce them.
	dagOut := publisher.IPFS("dag", "get", cidFile)
	var dagNode struct {
		Links []struct {
			Hash map[string]string `json:"Hash"`
		} `json:"Links"`
	}
	require.NoError(t, json.Unmarshal(dagOut.Stdout.Bytes(), &dagNode))
	require.Greater(t, len(dagNode.Links), 1, "file should have multiple chunks")
	cidChunk := dagNode.Links[0].Hash["/"]
	require.NotEmpty(t, cidChunk)

	// The async walk logs "fast-provide-dag: finished" with a
	// providedCIDs count on completion. A full walk of this file
	// visits the root plus every leaf chunk, so the count is much
	// larger than 2.
	providedRe := regexp.MustCompile(`"providedCIDs": (\d+)`)
	var providedCount int
	require.Eventually(t, func() bool {
		m := providedRe.FindStringSubmatch(publisher.Daemon.Stderr.String())
		if len(m) != 2 {
			return false
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return false
		}
		providedCount = n
		return true
	}, 30*time.Second, 200*time.Millisecond, "async fast-provide-dag walk did not log 'finished'")

	require.Greater(t, providedCount, 2,
		"providedCIDs=%d is too small for a full walk of the file DAG", providedCount)

	// End-to-end: the peer can find the publisher as a provider
	// for a chunk CID, which only the async walk could have
	// announced within the test window.
	pid := publisher.PeerID().String()
	var found bool
	for _, peer := range peers {
		for i := time.Duration(0); i*timeStep < timeout; i++ {
			res := peer.IPFS("routing", "findprovs", "-n=1", cidChunk)
			if res.Stdout.Trimmed() == pid {
				found = true
				break
			}
		}
	}
	require.True(t, found, "chunk %s not announced by the async walk", cidChunk)
}

// TestHTTPOnlyProviderWithSweepEnabled tests that provider records are correctly
// sent to HTTP routers when Routing.Type="custom" with only HTTP routers configured,
// even when Provide.DHT.SweepEnabled=true (the default since v0.39).
//
// This is a regression test for https://github.com/ipfs/kubo/issues/11089
func TestHTTPOnlyProviderWithSweepEnabled(t *testing.T) {
	t.Parallel()

	// Track provide requests received by the mock HTTP router
	var provideRequests atomic.Int32
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.Method == http.MethodPut || r.Method == http.MethodPost) &&
			strings.HasPrefix(r.URL.Path, "/routing/v1/providers") {
			provideRequests.Add(1)
			w.WriteHeader(http.StatusOK)
		} else if strings.HasPrefix(r.URL.Path, "/routing/v1/providers") && r.Method == http.MethodGet {
			// Return empty providers for findprovs
			w.Header().Set("Content-Type", "application/x-ndjson")
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	h := harness.NewT(t)
	node := h.NewNode().Init()

	// Explicitly set SweepEnabled=true (the default since v0.39, but be explicit for test clarity)
	node.SetIPFSConfig("Provide.DHT.SweepEnabled", true)
	node.SetIPFSConfig("Provide.Enabled", true)

	// Configure HTTP-only custom routing (no DHT) with explicit Routing.Type=custom
	routingConf := map[string]any{
		"Type": "custom", // Explicitly set Routing.Type=custom
		"Methods": map[string]any{
			"provide":        map[string]any{"RouterName": "HTTPRouter"},
			"get-ipns":       map[string]any{"RouterName": "HTTPRouter"},
			"put-ipns":       map[string]any{"RouterName": "HTTPRouter"},
			"find-peers":     map[string]any{"RouterName": "HTTPRouter"},
			"find-providers": map[string]any{"RouterName": "HTTPRouter"},
		},
		"Routers": map[string]any{
			"HTTPRouter": map[string]any{
				"Type": "http",
				"Parameters": map[string]any{
					"Endpoint": mockServer.URL,
				},
			},
		},
	}
	node.SetIPFSConfig("Routing", routingConf)
	node.StartDaemon()
	defer node.StopDaemon()

	// Add content and manually provide it
	cid := node.IPFSAddStr(time.Now().String())

	// Manual provide should succeed even without libp2p peers
	res := node.RunIPFS("routing", "provide", cid)
	// Check that the command succeeded (exit code 0) and no provide-related errors
	assert.Equal(t, 0, res.ExitCode(), "routing provide should succeed with HTTP-only routing and SweepEnabled=true")
	assert.NotContains(t, res.Stderr.String(), "cannot provide", "should not have provide errors")

	// Verify HTTP router received at least one provide request
	assert.Greater(t, provideRequests.Load(), int32(0),
		"HTTP router should have received provide requests")

	// Verify 'provide stat' works with HTTP-only routing (regression test for stats)
	statRes := node.RunIPFS("provide", "stat")
	assert.Equal(t, 0, statRes.ExitCode(), "provide stat should succeed with HTTP-only routing")
	assert.NotContains(t, statRes.Stderr.String(), "stats not available",
		"should not report stats unavailable")
	// LegacyProvider outputs "TotalReprovides:" in its stats
	assert.Contains(t, statRes.Stdout.String(), "TotalReprovides:",
		"should show legacy provider stats")
}

// TestProviderKeystoreDatastoreCompaction verifies that the SweepingProvider's
// keystore uses a datastore factory that creates separate physical datastores
// and reclaims disk space by deleting old datastores after each reset cycle.
//
// The keystore uses two alternating namespaces ("0" and "1") plus a "meta"
// namespace. The lifecycle is:
//  1. First start: namespace "0" is created as the initial active datastore
//  2. First reset (keystore sync at startup): "1" is created, data is written,
//     namespaces swap, "0" is destroyed from disk via os.RemoveAll
//  3. Restart: "1" and "meta" survive on disk
//  4. Second reset: "0" is recreated, namespaces swap, "1" is destroyed
func TestProviderKeystoreDatastorePurge(t *testing.T) {
	t.Parallel()

	h := harness.NewT(t)
	node := h.NewNode().Init()
	node.SetIPFSConfig("Provide.DHT.SweepEnabled", true)
	node.SetIPFSConfig("Provide.Enabled", true)
	node.SetIPFSConfig("Bootstrap", []string{})

	// Add content offline so the keystore has something to sync on startup.
	for i := range 5 {
		node.IPFSAddStr(fmt.Sprintf("keystore-compaction-test-%d", i))
	}

	keystoreBase := filepath.Join(node.Dir, "provider-keystore")
	ns0 := filepath.Join(keystoreBase, "0")
	ns1 := filepath.Join(keystoreBase, "1")

	// Directory should not exist before starting the daemon.
	_, err := os.Stat(keystoreBase)
	require.True(t, os.IsNotExist(err), "provider-keystore should not exist before daemon start")

	// --- First start: triggers keystore sync (ResetCids) ---
	// Init creates "0", then reset swaps to "1" and destroys "0".
	node.StartDaemon()

	require.Eventually(t, func() bool {
		return dirExists(ns1) && !dirExists(ns0)
	}, 30*time.Second, 200*time.Millisecond,
		"after first reset: ns1 should exist, ns0 should be destroyed")

	// --- Restart: triggers a second keystore sync (ResetCids) ---
	// Reset swaps back to "0" and destroys "1".
	node.StopDaemon()

	// Between restarts: ns1 survives on disk, ns0 does not.
	assert.True(t, dirExists(ns1), "ns1 should survive shutdown")
	assert.False(t, dirExists(ns0), "ns0 should not reappear between restarts")

	node.StartDaemon()

	require.Eventually(t, func() bool {
		return dirExists(ns0) && !dirExists(ns1)
	}, 30*time.Second, 200*time.Millisecond,
		"after second reset: ns0 should exist, ns1 should be destroyed")

	node.StopDaemon()
}

// TestProviderKeystoreMigrationPurge verifies that orphaned keystore data
// left in the shared repo datastore by older Kubo versions is purged on
// the first sweep-enabled daemon start. The migration is triggered by the
// absence of the <repo>/provider-keystore/ directory.
func TestProviderKeystoreMigrationPurge(t *testing.T) {
	t.Parallel()

	h := harness.NewT(t)
	node := h.NewNode().Init()
	node.SetIPFSConfig("Provide.DHT.SweepEnabled", true)
	node.SetIPFSConfig("Provide.Enabled", true)
	node.SetIPFSConfig("Bootstrap", []string{})

	keystoreBase := filepath.Join(node.Dir, "provider-keystore")

	// Pre-seed orphaned keystore data into the shared datastore, simulating
	// the layout produced by older Kubo that stored keystore entries inline.
	const numOrphans = 10
	for i := range numOrphans {
		node.DatastorePut(
			fmt.Sprintf("/provider/keystore/%d/fake-key-%d", i%2, i),
			fmt.Sprintf("orphan-%d", i),
		)
	}

	// The orphaned keys should be visible via diag datastore.
	count := node.DatastoreCount("/provider/keystore/")
	require.Equal(t, int64(numOrphans), count, "orphaned keys should be present before migration")

	// The provider-keystore directory must not exist yet (its absence
	// triggers the migration).
	require.False(t, dirExists(keystoreBase),
		"provider-keystore/ should not exist before first sweep-enabled start")

	// Start the daemon: this triggers the one-time migration purge.
	node.StartDaemon()
	node.StopDaemon()

	// After migration the seeded orphaned keys should be gone from the
	// shared datastore. The diag datastore count command mounts the
	// separate provider-keystore datastores, so we check for the specific
	// fake keys we seeded to confirm they were purged.
	for i := range numOrphans {
		key := fmt.Sprintf("/provider/keystore/%d/fake-key-%d", i%2, i)
		assert.False(t, node.DatastoreHasKey(key),
			"orphaned key %s should be purged after migration", key)
	}

	// The provider-keystore directory should now exist.
	assert.True(t, dirExists(keystoreBase),
		"provider-keystore/ should exist after sweep-enabled daemon ran")
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// TestProviderKeystoreSyncShutdownQuiet verifies two shutdown UX
// guarantees for a daemon running the sweeping provider with a
// pin-walking strategy (see ipfs/kubo#11292):
//
//  1. Shutdown-caused keystore-sync errors never appear at Error
//     level. The fix classifies keystore.ErrClosed and context
//     cancellation as shutdown-caused and logs at Debug as
//     "interrupted by shutdown" instead.
//  2. `ipfs pin ls --stream` running against the daemon returns a
//     meaningful error (no panic, no hang) when the daemon is
//     shutting down mid-stream.
//
// Determinism: with Provide.DHT.Interval=10ms the periodic
// reprovide goroutine runs syncKeystore back-to-back (ticks coalesce
// under the select), so it is always mid-sync when StopDaemon
// closes the keystore. The line-scan below fails on the exact
// Error+err=keystore-closed/context-canceled combination the old
// code emitted. Empirically this catches the regression on most
// runs (~3 of 5 on a fast workstation); the first few bug-free
// runs were verified by temporarily reverting core/node/provider.go.
func TestProviderKeystoreSyncShutdownQuiet(t *testing.T) {
	t.Parallel()

	h := harness.NewT(t)
	node := h.NewNode().Init()
	node.SetIPFSConfig("Provide.DHT.SweepEnabled", true)
	node.SetIPFSConfig("Provide.Enabled", true)
	node.SetIPFSConfig("Provide.Strategy", "pinned+mfs+entities")
	// Tight Interval: once the startup sync completes, the periodic
	// goroutine runs syncKeystore back-to-back (ticks coalesce under
	// the select), so it is always mid-sync when StopDaemon fires.
	// This makes the shutdown interrupt deterministic. Briefly
	// during startup the first periodic tick may overlap the startup
	// sync and emit "reset already in progress" at Error; the log
	// scan below explicitly ignores that unrelated class of error.
	node.SetIPFSConfig("Provide.DHT.Interval", "10ms")
	node.SetIPFSConfig("Bootstrap", []string{})

	// Seed recursive pins so the keystore sync has meaningful work.
	// Offline bulk add + bulk pin is much faster than per-file
	// IPFSAddStr calls for this count.
	const nPins = 500
	dir := t.TempDir()
	for i := range nPins {
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, fmt.Sprintf("f%04d", i)),
			fmt.Appendf(nil, "keystore-shutdown-content-%d", i),
			0o600,
		))
	}
	// --pin=false so the wrapping dir is not auto-pinned; each file
	// is then pinned individually below to get nPins separate pin
	// index entries (one big recursive pin would not exercise the
	// pin-index streamIndex walk the same way).
	addRes := node.IPFS("add", "-r", "-q", "--pin=false", dir)
	addedCIDs := strings.Split(strings.TrimSpace(addRes.Stdout.String()), "\n")
	require.GreaterOrEqual(t, len(addedCIDs), nPins, "expected at least %d CIDs from bulk add", nPins)
	pinArgs := append([]string{"pin", "add"}, addedCIDs[:nPins]...)
	node.IPFS(pinArgs...)

	node.StartDaemonWithReq(harness.RunRequest{
		CmdOpts: []harness.CmdOpt{
			harness.RunWithEnv(map[string]string{
				// Debug for the provider subsystem so the shutdown
				// Debug line is visible for post-hoc inspection.
				"GOLOG_LOG_LEVEL": "error,provider=debug",
			}),
		},
	}, "")

	// Wait for the startup sync to complete so periodic has sole
	// access to the keystore when we shut down.
	require.Eventually(t, func() bool {
		return strings.Contains(node.Daemon.Stderr.String(), "provider keystore sync completed")
	}, 30*time.Second, 50*time.Millisecond, "startup keystore sync should complete")

	// Let periodic reprovide fire several times.
	time.Sleep(1 * time.Second)

	// Kick off `ipfs pin ls --stream` against the live RPC. The
	// server-side channel is held by the pinner's streamIndex
	// goroutine; when StopDaemon below tears down the keystore and
	// datastore, the HTTP stream closes under the CLI, which must
	// exit cleanly with a meaningful error (no panic, no hang).
	pinLsDone := make(chan *harness.RunResult, 1)
	go func() {
		pinLsDone <- node.RunIPFS("pin", "ls", "--stream")
	}()
	// Brief delay so the pin ls RPC has started streaming.
	time.Sleep(100 * time.Millisecond)

	node.StopDaemon()

	// --- Daemon-side assertions ---

	daemonLog := node.Daemon.Stderr.String()

	// Scan for the specific bug pattern: an Error-level line from
	// the provider subsystem about "keystore sync" whose err field
	// is the shutdown-caused "keystore is closed" or "context
	// canceled". The fix routes those to Debug; only unrelated
	// errors (e.g. "reset already in progress" from test-induced
	// overlap) remain at Error and are ignored by this check.
	for line := range strings.SplitSeq(daemonLog, "\n") {
		if !strings.Contains(line, "\tERROR\t") {
			continue
		}
		if !strings.Contains(line, "provider keystore sync") {
			continue
		}
		if strings.Contains(line, `"err": "keystore is closed"`) ||
			strings.Contains(line, `"err": "context canceled"`) {
			t.Errorf("shutdown-caused keystore sync error should be logged at Debug, got Error:\n%s", line)
		}
	}

	// --- Client-side assertions (ipfs pin ls --stream) ---

	var pinLs *harness.RunResult
	select {
	case pinLs = <-pinLsDone:
	case <-time.After(15 * time.Second):
		t.Fatal("ipfs pin ls --stream did not return within 15s of daemon shutdown")
	}

	pinLsOut := pinLs.Stdout.String() + pinLs.Stderr.String()
	require.NotContains(t, pinLsOut, "panic:",
		"ipfs pin ls must not observe a daemon panic")
	// Either the stream drained before shutdown (exit 0) or the
	// server dropped it mid-stream (non-zero exit with a meaningful
	// error message). Silent non-zero exits are confusing and fail.
	if pinLs.ExitCode() != 0 {
		require.NotEmpty(t, strings.TrimSpace(pinLs.Stderr.String()),
			"pin ls exited non-zero but produced no error message")
	}
}
