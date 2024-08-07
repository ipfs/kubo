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

	pinLs := func(node *harness.Node, args ...string) []string {
		return strings.Split(node.IPFS(StrCat("pin", "ls", args)...).Stdout.Trimmed(), "\n")
	}

	t.Run("test pinning with names cli text output", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init()
		cidAStr := node.IPFSAddStr(RandomStr(1000), "--pin=false")
		cidBStr := node.IPFSAddStr(RandomStr(1000), "--pin=false")

		_ = node.IPFS("pin", "add", "--name", "testPin", cidAStr)

		outARegular := cidAStr + " recursive"
		outADetailed := outARegular + " testPin"
		outBRegular := cidBStr + " recursive"
		outBDetailed := outBRegular + " testPin"

		lsOut := pinLs(node, "-t=recursive")
		require.Contains(t, lsOut, outARegular)
		require.NotContains(t, lsOut, outADetailed)

		lsOut = pinLs(node, "-t=recursive", "--names")
		require.Contains(t, lsOut, outADetailed)
		require.NotContains(t, lsOut, outARegular)

		_ = node.IPFS("pin", "update", cidAStr, cidBStr)
		lsOut = pinLs(node, "-t=recursive", "--names")
		require.Contains(t, lsOut, outBDetailed)
		require.NotContains(t, lsOut, outADetailed)
	})

	t.Run("test listing pins with names that contain specific string", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init()
		cidAStr := node.IPFSAddStr(RandomStr(1000), "--pin=false")
		cidBStr := node.IPFSAddStr(RandomStr(1000), "--pin=false")
		cidCStr := node.IPFSAddStr(RandomStr(1000), "--pin=false")

		outA := cidAStr + " recursive testPin"
		outB := cidBStr + " recursive testPin"
		outC := cidCStr + " recursive randPin"

		// make sure both -n and --name work
		for _, nameParam := range []string{"--name", "-n"} {
			_ = node.IPFS("pin", "add", "--name", "testPin", cidAStr)
			lsOut := pinLs(node, "-t=recursive", nameParam+"=test")
			require.Contains(t, lsOut, outA)
			lsOut = pinLs(node, "-t=recursive", nameParam+"=randomLabel")
			require.NotContains(t, lsOut, outA)

			_ = node.IPFS("pin", "add", "--name", "testPin", cidBStr)
			lsOut = pinLs(node, "-t=recursive", nameParam+"=test")
			require.Contains(t, lsOut, outA)
			require.Contains(t, lsOut, outB)

			_ = node.IPFS("pin", "add", "--name", "randPin", cidCStr)
			lsOut = pinLs(node, "-t=recursive", nameParam+"=rand")
			require.NotContains(t, lsOut, outA)
			require.NotContains(t, lsOut, outB)
			require.Contains(t, lsOut, outC)

			lsOut = pinLs(node, "-t=recursive", nameParam+"=testPin")
			require.Contains(t, lsOut, outA)
			require.Contains(t, lsOut, outB)
			require.NotContains(t, lsOut, outC)
		}
	})

	t.Run("test overwriting pin with name", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init()
		cidStr := node.IPFSAddStr(RandomStr(1000), "--pin=false")

		outBefore := cidStr + " recursive A"
		outAfter := cidStr + " recursive B"

		_ = node.IPFS("pin", "add", "--name", "A", cidStr)
		lsOut := pinLs(node, "-t=recursive", "--names")
		require.Contains(t, lsOut, outBefore)
		require.NotContains(t, lsOut, outAfter)

		_ = node.IPFS("pin", "add", "--name", "B", cidStr)
		lsOut = pinLs(node, "-t=recursive", "--names")
		require.Contains(t, lsOut, outAfter)
		require.NotContains(t, lsOut, outBefore)
	})

	// JSON that is also the wire format of /api/v0
	t.Run("test pinning with names json output", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init()
		cidAStr := node.IPFSAddStr(RandomStr(1000), "--pin=false")
		cidBStr := node.IPFSAddStr(RandomStr(1000), "--pin=false")

		_ = node.IPFS("pin", "add", "--name", "testPinJson", cidAStr)

		outARegular := `"` + cidAStr + `":{"Type":"recursive"`
		outADetailed := outARegular + `,"Name":"testPinJson"`
		outBRegular := `"` + cidBStr + `":{"Type":"recursive"`
		outBDetailed := outBRegular + `,"Name":"testPinJson"`

		pinLs := func(args ...string) string {
			return node.IPFS(StrCat("pin", "ls", "--enc=json", args)...).Stdout.Trimmed()
		}

		lsOut := pinLs("-t=recursive")
		require.Contains(t, lsOut, outARegular)
		require.NotContains(t, lsOut, outADetailed)

		lsOut = pinLs("-t=recursive", "--names")
		require.Contains(t, lsOut, outADetailed)

		_ = node.IPFS("pin", "update", cidAStr, cidBStr)
		lsOut = pinLs("-t=recursive", "--names")
		require.Contains(t, lsOut, outBDetailed)
	})
}
