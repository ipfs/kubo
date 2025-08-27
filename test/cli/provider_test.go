package cli

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/ipfs/go-test/random"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvider(t *testing.T) {
	t.Parallel()

	initNodes := func(t *testing.T, n int, fn func(n *harness.Node)) harness.Nodes {
		nodes := harness.NewT(t).NewNodes(n).Init()
		nodes.ForEachPar(fn)
		return nodes.StartDaemons().Connect()
	}

	initNodesWithoutStart := func(t *testing.T, n int, fn func(n *harness.Node)) harness.Nodes {
		nodes := harness.NewT(t).NewNodes(n).Init()
		nodes.ForEachPar(fn)
		return nodes
	}

	expectNoProviders := func(t *testing.T, cid string, nodes ...*harness.Node) {
		for _, node := range nodes {
			res := node.IPFS("routing", "findprovs", "-n=1", cid)
			require.Empty(t, res.Stdout.String())
		}
	}

	expectProviders := func(t *testing.T, cid, expectedProvider string, nodes ...*harness.Node) {
		for _, node := range nodes {
			res := node.IPFS("routing", "findprovs", "-n=1", cid)
			require.Equal(t, expectedProvider, res.Stdout.Trimmed())
		}
	}

	t.Run("Provider.Enabled=true announces new CIDs created by ipfs add", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provider.Enabled", true)
		})
		defer nodes.StopDaemons()

		cid := nodes[0].IPFSAddStr(time.Now().String())
		expectProviders(t, cid, nodes[0].PeerID().String(), nodes[1:]...)
	})

	t.Run("Provider.Enabled=true announces new CIDs created by ipfs add --pin=false with default strategy", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provider.Enabled", true)
			// Default strategy is "all" which should provide even unpinned content
		})
		defer nodes.StopDaemons()

		cid := nodes[0].IPFSAddStr(time.Now().String(), "--pin=false")
		expectProviders(t, cid, nodes[0].PeerID().String(), nodes[1:]...)
	})

	t.Run("Provider.Enabled=true announces new CIDs created by ipfs block put --pin=false with default strategy", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provider.Enabled", true)
			// Default strategy is "all" which should provide unpinned content from block put
		})
		defer nodes.StopDaemons()

		data := random.Bytes(256)
		cid := nodes[0].IPFSBlockPut(bytes.NewReader(data), "--pin=false")
		expectProviders(t, cid, nodes[0].PeerID().String(), nodes[1:]...)
	})

	t.Run("Provider.Enabled=true announces new CIDs created by ipfs dag put --pin=false with default strategy", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provider.Enabled", true)
			// Default strategy is "all" which should provide unpinned content from dag put
		})
		defer nodes.StopDaemons()

		dagData := `{"hello": "world", "timestamp": "` + time.Now().String() + `"}`
		cid := nodes[0].IPFSDAGPut(bytes.NewReader([]byte(dagData)), "--pin=false")
		expectProviders(t, cid, nodes[0].PeerID().String(), nodes[1:]...)
	})

	t.Run("Provider.Enabled=false disables announcement of new CID from ipfs add", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provider.Enabled", false)
		})
		defer nodes.StopDaemons()

		cid := nodes[0].IPFSAddStr(time.Now().String())
		expectNoProviders(t, cid, nodes[1:]...)
	})

	t.Run("Provider.Enabled=false disables manual announcement via RPC command", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provider.Enabled", false)
		})
		defer nodes.StopDaemons()

		cid := nodes[0].IPFSAddStr(time.Now().String())
		res := nodes[0].RunIPFS("routing", "provide", cid)
		assert.Contains(t, res.Stderr.Trimmed(), "invalid configuration: Provider.Enabled is set to 'false'")
		assert.Equal(t, 1, res.ExitCode())

		expectNoProviders(t, cid, nodes[1:]...)
	})

	// Right now Provide and Reprovide are tied together
	t.Run("Reprovide.Interval=0 disables announcement of new CID too", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Reprovider.Interval", "0")
		})
		defer nodes.StopDaemons()

		cid := nodes[0].IPFSAddStr(time.Now().String())
		expectNoProviders(t, cid, nodes[1:]...)
	})

	// It is a lesser evil - forces users to fix their config and have some sort of interval
	t.Run("Manual Reprovider trigger does not work when periodic Reprovider is disabled", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Reprovider.Interval", "0")
		})
		defer nodes.StopDaemons()

		cid := nodes[0].IPFSAddStr(time.Now().String())

		expectNoProviders(t, cid, nodes[1:]...)

		res := nodes[0].RunIPFS("routing", "reprovide")
		assert.Contains(t, res.Stderr.Trimmed(), "invalid configuration: Reprovider.Interval is set to '0'")
		assert.Equal(t, 1, res.ExitCode())

		expectNoProviders(t, cid, nodes[1:]...)
	})

	// It is a lesser evil - forces users to fix their config and have some sort of interval
	t.Run("Manual Reprovider trigger does not work when Provider system is disabled", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Provider.Enabled", false)
		})
		defer nodes.StopDaemons()

		cid := nodes[0].IPFSAddStr(time.Now().String())

		expectNoProviders(t, cid, nodes[1:]...)

		res := nodes[0].RunIPFS("routing", "reprovide")
		assert.Contains(t, res.Stderr.Trimmed(), "invalid configuration: Provider.Enabled is set to 'false'")
		assert.Equal(t, 1, res.ExitCode())

		expectNoProviders(t, cid, nodes[1:]...)
	})

	t.Run("Provide with 'all' strategy", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Reprovider.Strategy", "all")
		})
		defer nodes.StopDaemons()

		cid := nodes[0].IPFSAddStr("all strategy")
		expectProviders(t, cid, nodes[0].PeerID().String(), nodes[1:]...)
	})

	t.Run("Provide with 'pinned' strategy", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Reprovider.Strategy", "pinned")
		})
		defer nodes.StopDaemons()

		// Add a non-pinned CID (should not be provided)
		cid := nodes[0].IPFSAddStr("pinned strategy", "--pin=false")
		expectNoProviders(t, cid, nodes[1:]...)

		// Pin the CID (should now be provided)
		nodes[0].IPFS("pin", "add", cid)
		expectProviders(t, cid, nodes[0].PeerID().String(), nodes[1:]...)
	})

	t.Run("Provide with 'pinned+mfs' strategy", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Reprovider.Strategy", "pinned+mfs")
		})
		defer nodes.StopDaemons()

		// Add a pinned CID (should be provided)
		cidPinned := nodes[0].IPFSAddStr("pinned content")
		cidUnpinned := nodes[0].IPFSAddStr("unpinned content", "--pin=false")
		cidMFS := nodes[0].IPFSAddStr("mfs content", "--pin=false")
		nodes[0].IPFS("files", "cp", "/ipfs/"+cidMFS, "/myfile")

		n0pid := nodes[0].PeerID().String()
		expectProviders(t, cidPinned, n0pid, nodes[1:]...)
		expectNoProviders(t, cidUnpinned, nodes[1:]...)
		expectProviders(t, cidMFS, n0pid, nodes[1:]...)
	})

	t.Run("Provide with 'roots' strategy", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Reprovider.Strategy", "roots")
		})
		defer nodes.StopDaemons()

		// Add a root CID (should be provided)
		cidRoot := nodes[0].IPFSAddStr("roots strategy", "-w", "-Q")
		// the same without wrapping should give us a child node.
		cidChild := nodes[0].IPFSAddStr("root strategy", "--pin=false")

		expectProviders(t, cidRoot, nodes[0].PeerID().String(), nodes[1:]...)
		expectNoProviders(t, cidChild, nodes[1:]...)
	})

	t.Run("Provide with 'mfs' strategy", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Reprovider.Strategy", "mfs")
		})
		defer nodes.StopDaemons()

		// Add a file to MFS (should be provided)
		data := random.Bytes(1000)
		cid := nodes[0].IPFSAdd(bytes.NewReader(data), "-Q")

		// not yet in MFS
		expectNoProviders(t, cid, nodes[1:]...)

		nodes[0].IPFS("files", "cp", "/ipfs/"+cid, "/myfile")
		expectProviders(t, cid, nodes[0].PeerID().String(), nodes[1:]...)
	})

	t.Run("Reprovides with 'all' strategy when strategy is '' (empty)", func(t *testing.T) {
		t.Parallel()

		nodes := initNodesWithoutStart(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Reprovider.Strategy", "")
		})

		cid := nodes[0].IPFSAddStr(time.Now().String())

		nodes = nodes.StartDaemons().Connect()
		defer nodes.StopDaemons()
		expectNoProviders(t, cid, nodes[1:]...)

		nodes[0].IPFS("routing", "reprovide")

		expectProviders(t, cid, nodes[0].PeerID().String(), nodes[1:]...)
	})

	t.Run("Reprovides with 'all' strategy", func(t *testing.T) {
		t.Parallel()

		nodes := initNodesWithoutStart(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Reprovider.Strategy", "all")
		})

		cid := nodes[0].IPFSAddStr(time.Now().String())

		nodes = nodes.StartDaemons().Connect()
		defer nodes.StopDaemons()
		expectNoProviders(t, cid, nodes[1:]...)

		nodes[0].IPFS("routing", "reprovide")

		expectProviders(t, cid, nodes[0].PeerID().String(), nodes[1:]...)
	})

	t.Run("Reprovides with 'pinned' strategy", func(t *testing.T) {
		t.Parallel()

		foo := random.Bytes(1000)
		bar := random.Bytes(1000)

		nodes := initNodesWithoutStart(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Reprovider.Strategy", "pinned")
		})

		// Add a pin while offline so it cannot be provided
		cidBarDir := nodes[0].IPFSAdd(bytes.NewReader(bar), "-Q", "-w")

		nodes = nodes.StartDaemons().Connect()
		defer nodes.StopDaemons()

		// Add content without pinning while daemon line
		cidFoo := nodes[0].IPFSAdd(bytes.NewReader(foo), "--pin=false")
		cidBar := nodes[0].IPFSAdd(bytes.NewReader(bar), "--pin=false")

		// Nothing should have been provided. The pin was offline, and
		// the others should not be provided per the strategy.
		expectNoProviders(t, cidFoo, nodes[1:]...)
		expectNoProviders(t, cidBar, nodes[1:]...)
		expectNoProviders(t, cidBarDir, nodes[1:]...)

		nodes[0].IPFS("routing", "reprovide")

		// cidFoo is not pinned so should not be provided.
		expectNoProviders(t, cidFoo, nodes[1:]...)
		// cidBar gets provided by being a child from cidBarDir even though we added with pin=false.
		expectProviders(t, cidBar, nodes[0].PeerID().String(), nodes[1:]...)
		expectProviders(t, cidBarDir, nodes[0].PeerID().String(), nodes[1:]...)
	})

	t.Run("Reprovides with 'roots' strategy", func(t *testing.T) {
		t.Parallel()

		foo := random.Bytes(1000)
		bar := random.Bytes(1000)

		nodes := initNodesWithoutStart(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Reprovider.Strategy", "roots")
		})
		n0pid := nodes[0].PeerID().String()

		// Add a pin. Only root should get pinned but not provided
		// because node not started
		cidBarDir := nodes[0].IPFSAdd(bytes.NewReader(bar), "-Q", "-w")

		nodes = nodes.StartDaemons().Connect()
		defer nodes.StopDaemons()

		cidFoo := nodes[0].IPFSAdd(bytes.NewReader(foo))
		cidBar := nodes[0].IPFSAdd(bytes.NewReader(bar), "--pin=false")

		// cidFoo will get provided per the strategy but cidBar will not.
		expectProviders(t, cidFoo, n0pid, nodes[1:]...)
		expectNoProviders(t, cidBar, nodes[1:]...)

		nodes[0].IPFS("routing", "reprovide")

		expectProviders(t, cidFoo, n0pid, nodes[1:]...)
		expectNoProviders(t, cidBar, nodes[1:]...)
		expectProviders(t, cidBarDir, n0pid, nodes[1:]...)
	})

	t.Run("Reprovides with 'mfs' strategy", func(t *testing.T) {
		t.Parallel()

		bar := random.Bytes(1000)

		nodes := initNodesWithoutStart(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Reprovider.Strategy", "mfs")
		})
		n0pid := nodes[0].PeerID().String()

		// add something and lets put it in MFS
		cidBar := nodes[0].IPFSAdd(bytes.NewReader(bar), "--pin=false", "-Q")
		nodes[0].IPFS("files", "cp", "/ipfs/"+cidBar, "/myfile")

		nodes = nodes.StartDaemons().Connect()
		defer nodes.StopDaemons()

		// cidBar is in MFS but not provided
		expectNoProviders(t, cidBar, nodes[1:]...)

		nodes[0].IPFS("routing", "reprovide")

		// And now is provided
		expectProviders(t, cidBar, n0pid, nodes[1:]...)
	})

	t.Run("Reprovides with 'pinned+mfs' strategy", func(t *testing.T) {
		t.Parallel()

		nodes := initNodesWithoutStart(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Reprovider.Strategy", "pinned+mfs")
		})
		n0pid := nodes[0].PeerID().String()

		// Add a pinned CID (should be provided)
		cidPinned := nodes[0].IPFSAddStr("pinned content", "--pin=true")
		// Add a CID to MFS (should be provided)
		cidMFS := nodes[0].IPFSAddStr("mfs content")
		nodes[0].IPFS("files", "cp", "/ipfs/"+cidMFS, "/myfile")
		// Add a CID that is neither pinned nor in MFS (should not be provided)
		cidNeither := nodes[0].IPFSAddStr("neither content", "--pin=false")

		nodes = nodes.StartDaemons().Connect()
		defer nodes.StopDaemons()

		// Trigger reprovide
		nodes[0].IPFS("routing", "reprovide")

		// Check that pinned CID is provided
		expectProviders(t, cidPinned, n0pid, nodes[1:]...)
		// Check that MFS CID is provided
		expectProviders(t, cidMFS, n0pid, nodes[1:]...)
		// Check that neither CID is not provided
		expectNoProviders(t, cidNeither, nodes[1:]...)
	})

	t.Run("provide clear command removes items from provide queue", func(t *testing.T) {
		t.Parallel()

		nodes := harness.NewT(t).NewNodes(1).Init()
		nodes.ForEachPar(func(n *harness.Node) {
			n.SetIPFSConfig("Provider.Enabled", true)
			n.SetIPFSConfig("Reprovider.Interval", "22h")
			n.SetIPFSConfig("Reprovider.Strategy", "all")
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
			n.SetIPFSConfig("Provider.Enabled", true)
			n.SetIPFSConfig("Reprovider.Interval", "22h")
			n.SetIPFSConfig("Reprovider.Strategy", "all")
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
			n.SetIPFSConfig("Provider.Enabled", false)
			n.SetIPFSConfig("Reprovider.Interval", "22h")
			n.SetIPFSConfig("Reprovider.Strategy", "all")
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
			n.SetIPFSConfig("Provider.Enabled", true)
			n.SetIPFSConfig("Reprovider.Interval", "22h")
			n.SetIPFSConfig("Reprovider.Strategy", "all")
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
