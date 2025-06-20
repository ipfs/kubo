package cli

import (
	"bytes"
	"testing"
	"time"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/ipfs/kubo/test/cli/testutils"
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
		// Reprovide as initialProviderDelay still ongoing
		res := nodes[0].IPFS("routing", "reprovide")
		require.NoError(t, res.Err)
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

		cid := nodes[0].IPFSAddStr(time.Now().String(), "--offline")

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

		cid := nodes[0].IPFSAddStr(time.Now().String(), "--offline")

		expectNoProviders(t, cid, nodes[1:]...)

		res := nodes[0].RunIPFS("routing", "reprovide")
		assert.Contains(t, res.Stderr.Trimmed(), "invalid configuration: Provider.Enabled is set to 'false'")
		assert.Equal(t, 1, res.ExitCode())

		expectNoProviders(t, cid, nodes[1:]...)
	})

	t.Run("Reprovides with 'all' strategy", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Reprovider.Strategy", "all")
		})
		defer nodes.StopDaemons()

		cid := nodes[0].IPFSAddStr(time.Now().String(), "--local")

		expectNoProviders(t, cid, nodes[1:]...)

		nodes[0].IPFS("routing", "reprovide")

		expectProviders(t, cid, nodes[0].PeerID().String(), nodes[1:]...)
	})

	t.Run("Reprovides with 'flat' strategy", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Reprovider.Strategy", "flat")
		})
		defer nodes.StopDaemons()

		cid := nodes[0].IPFSAddStr(time.Now().String(), "--local")

		expectNoProviders(t, cid, nodes[1:]...)

		nodes[0].IPFS("routing", "reprovide")

		expectProviders(t, cid, nodes[0].PeerID().String(), nodes[1:]...)
	})

	t.Run("Reprovides with 'pinned' strategy", func(t *testing.T) {
		t.Parallel()

		foo := testutils.RandomBytes(1000)
		bar := testutils.RandomBytes(1000)

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Reprovider.Strategy", "pinned")
		})
		defer nodes.StopDaemons()

		cidFoo := nodes[0].IPFSAdd(bytes.NewReader(foo), "--offline", "--pin=false")
		cidBar := nodes[0].IPFSAdd(bytes.NewReader(bar), "--offline", "--pin=false")
		cidBarDir := nodes[0].IPFSAdd(bytes.NewReader(bar), "-Q", "--offline", "-w")

		expectNoProviders(t, cidFoo, nodes[1:]...)
		expectNoProviders(t, cidBar, nodes[1:]...)
		expectNoProviders(t, cidBarDir, nodes[1:]...)

		nodes[0].IPFS("routing", "reprovide")

		expectNoProviders(t, cidFoo, nodes[1:]...)
		expectProviders(t, cidBar, nodes[0].PeerID().String(), nodes[1:]...)
		expectProviders(t, cidBarDir, nodes[0].PeerID().String(), nodes[1:]...)
	})

	t.Run("Reprovides with 'roots' strategy", func(t *testing.T) {
		t.Parallel()

		foo := testutils.RandomBytes(1000)
		bar := testutils.RandomBytes(1000)
		baz := testutils.RandomBytes(1000)

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Reprovider.Strategy", "roots")
		})
		defer nodes.StopDaemons()

		cidFoo := nodes[0].IPFSAdd(bytes.NewReader(foo), "--offline", "--pin=false")
		cidBar := nodes[0].IPFSAdd(bytes.NewReader(bar), "--offline", "--pin=false")
		cidBaz := nodes[0].IPFSAdd(bytes.NewReader(baz), "--offline")
		cidBarDir := nodes[0].IPFSAdd(bytes.NewReader(bar), "-Q", "--offline", "-w")

		expectNoProviders(t, cidFoo, nodes[1:]...)
		expectNoProviders(t, cidBar, nodes[1:]...)
		expectNoProviders(t, cidBarDir, nodes[1:]...)

		nodes[0].IPFS("routing", "reprovide")

		expectNoProviders(t, cidFoo, nodes[1:]...)
		expectNoProviders(t, cidBar, nodes[1:]...)
		expectProviders(t, cidBaz, nodes[0].PeerID().String(), nodes[1:]...)
		expectProviders(t, cidBarDir, nodes[0].PeerID().String(), nodes[1:]...)
	})

}
