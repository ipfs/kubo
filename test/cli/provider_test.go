package cli

import (
	"bytes"
	"testing"
	"time"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/ipfs/kubo/test/cli/testutils"
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

	t.Run("Basic Providing", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Experimental.StrategicProviding", false)
		})
		defer nodes.StopDaemons()

		cid := nodes[0].IPFSAddStr(time.Now().String())
		// Reprovide as initialProviderDelay still ongoing
		res := nodes[0].IPFS("bitswap", "reprovide")
		require.NoError(t, res.Err)
		expectProviders(t, cid, nodes[0].PeerID().String(), nodes[1:]...)
	})

	t.Run("Basic Strategic Providing", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Experimental.StrategicProviding", true)
		})
		defer nodes.StopDaemons()

		cid := nodes[0].IPFSAddStr(time.Now().String())
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

		nodes[0].IPFS("bitswap", "reprovide")

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

		nodes[0].IPFS("bitswap", "reprovide")

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

		nodes[0].IPFS("bitswap", "reprovide")

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

		nodes[0].IPFS("bitswap", "reprovide")

		expectNoProviders(t, cidFoo, nodes[1:]...)
		expectNoProviders(t, cidBar, nodes[1:]...)
		expectProviders(t, cidBaz, nodes[0].PeerID().String(), nodes[1:]...)
		expectProviders(t, cidBarDir, nodes[0].PeerID().String(), nodes[1:]...)
	})

	t.Run("Providing works without ticking", func(t *testing.T) {
		t.Parallel()

		nodes := initNodes(t, 2, func(n *harness.Node) {
			n.SetIPFSConfig("Reprovider.Interval", "0")
		})
		defer nodes.StopDaemons()

		cid := nodes[0].IPFSAddStr(time.Now().String(), "--offline")

		expectNoProviders(t, cid, nodes[1:]...)

		nodes[0].IPFS("bitswap", "reprovide")

		expectProviders(t, cid, nodes[0].PeerID().String(), nodes[1:]...)
	})
}
