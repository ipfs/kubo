package cli

import (
	"path/filepath"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mountAt returns the datastore definition mounted at mountpoint in a "mount" spec.
func mountAt(t *testing.T, spec map[string]any, mountpoint string) map[string]any {
	t.Helper()
	require.Equal(t, "mount", spec["type"])
	mounts, ok := spec["mounts"].([]any)
	require.True(t, ok, "mounts is not a list: %#v", spec["mounts"])
	for _, m := range mounts {
		mount, ok := m.(map[string]any)
		require.True(t, ok, "mount is not an object: %#v", m)
		if mount["mountpoint"] == mountpoint {
			return mount
		}
	}
	t.Fatalf("no mount at %q in %#v", mountpoint, spec)
	return nil
}

// measureChild returns the datastore wrapped by a "measure" mount.
func measureChild(t *testing.T, mount map[string]any) map[string]any {
	t.Helper()
	require.Equal(t, "measure", mount["type"])
	child, ok := mount["child"].(map[string]any)
	require.True(t, ok, "measure has no child: %#v", mount)
	return child
}

// The flatfs-pebbleds profiles keep blocks in flatfs and everything else in pebble.
func TestFlatfsPebbledsProfile(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		profile  string
		measured bool
	}{
		{profile: "flatfs-pebbleds", measured: false},
		{profile: "flatfs-pebbleds-measure", measured: true},
	} {
		t.Run(tc.profile, func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init("--profile=" + tc.profile)

			spec := node.ReadConfig().Datastore.Spec
			blocks := mountAt(t, spec, "/blocks")
			root := mountAt(t, spec, "/")
			if tc.measured {
				blocks = measureChild(t, blocks)
				root = measureChild(t, root)
			}
			assert.Equal(t, "flatfs", blocks["type"])
			assert.Equal(t, "pebbleds", root["type"])
			assert.EqualValues(t, pebble.FormatNewest, root["formatMajorVersion"], "pebble format is pinned at init")

			// init opens the datastores, so the flatfs and pebble directories
			// exist and no leveldb directory was created
			assert.DirExists(t, filepath.Join(node.Dir, "blocks"))
			assert.DirExists(t, filepath.Join(node.Dir, "pebbleds"))
			assert.NoDirExists(t, filepath.Join(node.Dir, "datastore"))

			// pins live in pebble, blocks in flatfs; both survive a restart
			node.StartDaemon("--offline")
			cid := node.IPFSAddStr("hello " + tc.profile)
			node.StopDaemon()
			node.StartDaemon("--offline")
			defer node.StopDaemon()

			assert.Equal(t, "hello "+tc.profile, node.IPFS("cat", cid).Stdout.String())
			assert.Contains(t, node.IPFS("pin", "ls", "-t", "recursive", cid).Stdout.String(), cid)

			// only the measure variant exposes per-datastore metrics
			resp := node.APIClient().Get("/debug/metrics/prometheus")
			require.Equal(t, 200, resp.StatusCode)
			for _, prefix := range []string{"flatfs_datastore_", "pebble_datastore_"} {
				if tc.measured {
					assert.Contains(t, resp.Body, prefix)
				} else {
					assert.NotContains(t, resp.Body, prefix)
				}
			}
		})
	}
}

// flatfs and flatfs-measure are aliases of the flatfs-levelds profiles.
func TestFlatfsProfileAliases(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		alias, canonical string
	}{
		{alias: "flatfs", canonical: "flatfs-levelds"},
		{alias: "flatfs-measure", canonical: "flatfs-levelds-measure"},
	} {
		t.Run(tc.alias, func(t *testing.T) {
			t.Parallel()
			nodes := harness.NewT(t).NewNodes(2)
			nodes[0].Init("--profile=" + tc.canonical)
			nodes[1].Init("--profile=" + tc.alias)

			canonical := nodes[0].ReadConfig().Datastore.Spec
			alias := nodes[1].ReadConfig().Datastore.Spec
			assert.Equal(t, canonical, alias)
			assert.DirExists(t, filepath.Join(nodes[1].Dir, "blocks"))
			assert.DirExists(t, filepath.Join(nodes[1].Dir, "datastore"))
		})
	}
}

// flatfs-levelds is the default layout: blocks in flatfs, everything else in leveldb.
func TestFlatfsLeveldsProfileIsDefault(t *testing.T) {
	t.Parallel()
	nodes := harness.NewT(t).NewNodes(2)
	nodes[0].Init()
	nodes[1].Init("--profile=flatfs-levelds")

	spec := nodes[1].ReadConfig().Datastore.Spec
	assert.Equal(t, nodes[0].ReadConfig().Datastore.Spec, spec)
	assert.Equal(t, "flatfs", mountAt(t, spec, "/blocks")["type"])
	assert.Equal(t, "levelds", mountAt(t, spec, "/")["type"])
}
