package core_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/plugin"
	"github.com/ipfs/kubo/plugin/plugins/pebbleds"
	"github.com/ipfs/kubo/repo/fsrepo"
	"github.com/stretchr/testify/require"
)

// TestNilRoutingKeystoreReopen guards against a provider-keystore datastore
// leak. With Routing.Type=none the provider is a NoopProvider, but the keystore
// still eagerly opens its on-disk datastore under <repo>/provider-keystore/.
// If nothing closes that datastore, its file lock blocks reopening the same
// repo in the same process, which breaks anything that embeds kubo and restarts
// a node (tests, apps). The daemon CLI hides this because the process exits and
// the OS releases the lock.
func TestNilRoutingKeystoreReopen(t *testing.T) {
	registerPebbleDatastore(t)

	repoPath := t.TempDir()

	cfg, err := config.Init(io.Discard, 2048)
	require.NoError(t, err)
	// Bind ephemeral localhost ports so the test does not collide with a
	// daemon already listening on the default swarm port.
	cfg.Addresses.Swarm = []string{"/ip4/127.0.0.1/tcp/0"}
	cfg.Bootstrap = []string{}
	// A single on-disk datastore (not a MapDatastore) is required: the leak
	// is an unreleased datastore lock, and an in-memory datastore takes none.
	cfg.Datastore.Spec = map[string]any{
		"type":               "pebbleds",
		"prefix":             "pebble.datastore",
		"path":               "pebbleds",
		"formatMajorVersion": int(pebble.FormatNewest),
	}

	require.NoError(t, fsrepo.Init(repoPath, cfg))

	startStop := func() {
		r, err := fsrepo.Open(repoPath)
		require.NoError(t, err)

		node, err := core.NewNode(t.Context(), &core.BuildCfg{
			Online:  true,
			Routing: libp2p.NilRouterOption,
			Repo:    r,
		})
		require.NoError(t, err)
		require.NoError(t, node.Close())
	}

	// First run creates and opens the keystore datastore.
	startStop()

	// The keystore datastore must have been closed on the first run. Before the
	// fix this second open fails with a lock error ("resource temporarily
	// unavailable" on Unix, "being used by another process" on Windows).
	startStop()

	// Sanity check: the keystore directory was actually created on disk, so the
	// test really exercised an on-disk (lockable) datastore rather than silently
	// falling back to an in-memory one.
	keystoreDir := filepath.Join(repoPath, "provider-keystore")
	_, err = os.Stat(keystoreDir)
	require.NoError(t, err, "provider-keystore should exist after a node ran")
}

// registerPebbleDatastore registers the pebbleds datastore handler with fsrepo.
// AddDatastoreConfigHandler errors if the handler is already registered, which
// is harmless here, so a duplicate registration is tolerated.
func registerPebbleDatastore(t *testing.T) {
	t.Helper()
	dsPlugin, ok := pebbleds.Plugins[0].(plugin.PluginDatastore)
	require.True(t, ok, "pebbleds plugin should implement PluginDatastore")
	err := fsrepo.AddDatastoreConfigHandler(dsPlugin.DatastoreTypeName(), dsPlugin.DatastoreConfigParser())
	if err != nil && !strings.Contains(err.Error(), "already have a datastore") {
		t.Fatalf("registering pebbleds datastore handler: %v", err)
	}
}
