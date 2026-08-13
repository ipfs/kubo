package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ipfs/boxo/filestore"
	"github.com/ipfs/boxo/keystore"
	"github.com/ipfs/go-datastore"
	syncds "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/repo"
	"github.com/stretchr/testify/require"
)

// The peerstore self-entry is what feeds the signed peer record, so a node
// listening only on loopback publishes nothing once non-public addresses are
// filtered out. host.Addrs() keeps reporting them either way: the option
// governs what the node announces, not what it listens on.
func TestInternalNonPublicAddrPublishing(t *testing.T) {
	for _, tc := range []struct {
		name              string
		flag              config.Flag
		wantSelfPublished bool
	}{
		{"unset announces loopback, matching the go-libp2p default", config.Default, true},
		{"true announces loopback, as a LAN-only node needs", config.True, true},
		{"false keeps loopback out of the published set", config.False, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			node := buildOnlineLoopbackNode(t, tc.flag)

			require.NotEmpty(t, node.PeerHost.Addrs(), "the host still listens on loopback")

			published := waitForSelfAddrs(t, node, tc.wantSelfPublished)
			if tc.wantSelfPublished {
				require.NotEmpty(t, published, "loopback should be published")
			} else {
				require.Empty(t, published, "loopback should not be published")
			}
		})
	}
}

func buildOnlineLoopbackNode(t *testing.T, flag config.Flag) *IpfsNode {
	t.Helper()

	ds := syncds.MutexWrap(datastore.NewMapDatastore())
	c := config.Config{}
	c.Identity = testIdentity
	c.Addresses.Swarm = []string{"/ip4/127.0.0.1/tcp/0"}
	c.Internal.NonPublicAddrPublishing = flag
	c.Bootstrap = []string{}

	node, err := NewNode(t.Context(), &BuildCfg{
		Repo: &repo.Mock{
			C: c,
			D: ds,
			K: keystore.NewMemKeystore(),
			F: filestore.NewFileManager(ds, filepath.Dir(os.TempDir())),
		},
		Online:  true,
		Routing: libp2p.NilRouterOption,
	})
	require.NoError(t, err)
	t.Cleanup(func() { node.Close() })
	return node
}

// waitForSelfAddrs polls the peerstore self-entry, which the address manager
// fills asynchronously once the host starts listening.
func waitForSelfAddrs(t *testing.T, n *IpfsNode, wantAddrs bool) []string {
	t.Helper()

	var addrs []string
	require.Eventually(t, func() bool {
		addrs = nil
		for _, a := range n.Peerstore.Addrs(n.Identity) {
			addrs = append(addrs, a.String())
		}
		return (len(addrs) > 0) == wantAddrs
	}, 5*time.Second, 50*time.Millisecond)
	return addrs
}
