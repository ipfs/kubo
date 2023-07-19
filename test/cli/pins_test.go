package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/test/cli/harness"
	. "github.com/ipfs/kubo/test/cli/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testPinsArgs struct {
	runDaemon bool
	pinArg    string
	lsArg     string
	baseArg   string
}

func testPins(t *testing.T, args testPinsArgs) {
	t.Run(fmt.Sprintf("test pins with args=%+v", args), func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		if args.runDaemon {
			node.StartDaemon("--offline")
		}

		strs := []string{"a", "b", "c", "d", "e", "f", "g"}
		dataToCid := map[string]string{}
		cids := []string{}

		ipfsAdd := func(t *testing.T, content string) string {
			cidStr := node.IPFSAddStr(content, StrCat(args.baseArg, "--pin=false")...)

			_, err := cid.Decode(cidStr)
			require.NoError(t, err)
			dataToCid[content] = cidStr
			cids = append(cids, cidStr)
			return cidStr
		}

		ipfsPinAdd := func(cids []string) []string {
			input := strings.Join(cids, "\n")
			return node.PipeStrToIPFS(input, StrCat("pin", "add", args.pinArg, args.baseArg)...).Stdout.Lines()
		}

		ipfsPinLS := func() string {
			return node.IPFS(StrCat("pin", "ls", args.lsArg, args.baseArg)...).Stdout.Trimmed()
		}

		for _, s := range strs {
			ipfsAdd(t, s)
		}

		// these subtests run sequentially since they depend on state

		t.Run("check output of pin command", func(t *testing.T) {
			resLines := ipfsPinAdd(cids)

			for i, s := range resLines {
				assert.Equal(t,
					fmt.Sprintf("pinned %s recursively", cids[i]),
					s,
				)
			}
		})

		t.Run("pin verify should succeed", func(t *testing.T) {
			node.IPFS("pin", "verify")
		})

		t.Run("'pin verify --verbose' should include all the cids", func(t *testing.T) {
			verboseVerifyOut := node.IPFS(StrCat("pin", "verify", "--verbose", args.baseArg)...).Stdout.String()
			for _, cid := range cids {
				assert.Contains(t, verboseVerifyOut, fmt.Sprintf("%s ok", cid))
			}

		})
		t.Run("ls output should contain the cids", func(t *testing.T) {
			lsOut := ipfsPinLS()
			for _, cid := range cids {
				assert.Contains(t, lsOut, cid)
			}
		})

		t.Run("check 'pin ls hash' output", func(t *testing.T) {
			lsHashOut := node.IPFS(StrCat("pin", "ls", args.lsArg, args.baseArg, dataToCid["b"])...)
			lsHashOutStr := lsHashOut.Stdout.String()
			assert.Equal(t, fmt.Sprintf("%s recursive\n", dataToCid["b"]), lsHashOutStr)
		})

		t.Run("unpinning works", func(t *testing.T) {
			node.PipeStrToIPFS(strings.Join(cids, "\n"), "pin", "rm")
		})

		t.Run("test pin update", func(t *testing.T) {
			cidA := dataToCid["a"]
			cidB := dataToCid["b"]

			ipfsPinAdd([]string{cidA})
			beforeUpdate := ipfsPinLS()

			assert.Contains(t, beforeUpdate, cidA)
			assert.NotContains(t, beforeUpdate, cidB)

			node.IPFS("pin", "update", "--unpin=true", cidA, cidB)
			afterUpdate := ipfsPinLS()

			assert.NotContains(t, afterUpdate, cidA)
			assert.Contains(t, afterUpdate, cidB)

			node.IPFS("pin", "update", "--unpin=true", cidB, cidB)
			afterIdempotentUpdate := ipfsPinLS()

			assert.Contains(t, afterIdempotentUpdate, cidB)

			node.IPFS("pin", "rm", cidB)
		})
	})
}

func testPinsErrorReporting(t *testing.T, args testPinsArgs) {
	t.Run(fmt.Sprintf("test pins error reporting with args=%+v", args), func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		if args.runDaemon {
			node.StartDaemon("--offline")
		}
		randomCID := "Qme8uX5n9hn15pw9p6WcVKoziyyC9LXv4LEgvsmKMULjnV"
		res := node.RunIPFS(StrCat("pin", "add", args.pinArg, randomCID)...)
		assert.NotEqual(t, 0, res.ExitErr.ExitCode())
		assert.Contains(t, res.Stderr.String(), "ipld: could not find")
	})
}

func testPinDAG(t *testing.T, args testPinsArgs) {
	t.Run(fmt.Sprintf("test pin DAG with args=%+v", args), func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		node := h.NewNode().Init()
		if args.runDaemon {
			node.StartDaemon("--offline")
		}
		bytes := RandomBytes(1 << 20) // 1 MiB
		tmpFile := h.WriteToTemp(string(bytes))
		cid := node.IPFS(StrCat("add", args.pinArg, "--pin=false", "-q", tmpFile)...).Stdout.Trimmed()

		node.IPFS("pin", "add", "--recursive=true", cid)
		node.IPFS("pin", "rm", cid)

		// remove part of the DAG
		part := node.IPFS("refs", cid).Stdout.Lines()[0]
		node.IPFS("block", "rm", part)

		res := node.RunIPFS("pin", "add", "--recursive=true", cid)
		assert.NotEqual(t, 0, res)
		assert.Contains(t, res.Stderr.String(), "ipld: could not find")
	})
}

func testPinProgress(t *testing.T, args testPinsArgs) {
	t.Run(fmt.Sprintf("test pin progress with args=%+v", args), func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		node := h.NewNode().Init()

		if args.runDaemon {
			node.StartDaemon("--offline")
		}

		bytes := RandomBytes(1 << 20) // 1 MiB
		tmpFile := h.WriteToTemp(string(bytes))
		cid := node.IPFS(StrCat("add", args.pinArg, "--pin=false", "-q", tmpFile)...).Stdout.Trimmed()

		res := node.RunIPFS("pin", "add", "--progress", cid)
		node.Runner.AssertNoError(res)

		assert.Contains(t, res.Stderr.String(), " 5 nodes")
	})
}

func TestPins(t *testing.T) {
	t.Parallel()
	t.Run("test pinning without daemon running", func(t *testing.T) {
		t.Parallel()
		testPinsErrorReporting(t, testPinsArgs{})
		testPinsErrorReporting(t, testPinsArgs{pinArg: "--progress"})
		testPinDAG(t, testPinsArgs{})
		testPinDAG(t, testPinsArgs{pinArg: "--raw-leaves"})
		testPinProgress(t, testPinsArgs{})
		testPins(t, testPinsArgs{})
		testPins(t, testPinsArgs{pinArg: "--progress"})
		testPins(t, testPinsArgs{pinArg: "--progress", lsArg: "--stream"})
		testPins(t, testPinsArgs{baseArg: "--cid-base=base32"})
		testPins(t, testPinsArgs{lsArg: "--stream", baseArg: "--cid-base=base32"})

	})

	t.Run("test pinning with daemon running without network", func(t *testing.T) {
		t.Parallel()
		testPinsErrorReporting(t, testPinsArgs{runDaemon: true})
		testPinsErrorReporting(t, testPinsArgs{runDaemon: true, pinArg: "--progress"})
		testPinDAG(t, testPinsArgs{runDaemon: true})
		testPinDAG(t, testPinsArgs{runDaemon: true, pinArg: "--raw-leaves"})
		testPinProgress(t, testPinsArgs{runDaemon: true})
		testPins(t, testPinsArgs{runDaemon: true})
		testPins(t, testPinsArgs{runDaemon: true, pinArg: "--progress"})
		testPins(t, testPinsArgs{runDaemon: true, pinArg: "--progress", lsArg: "--stream"})
		testPins(t, testPinsArgs{runDaemon: true, baseArg: "--cid-base=base32"})
		testPins(t, testPinsArgs{runDaemon: true, lsArg: "--stream", baseArg: "--cid-base=base32"})
	})
}
