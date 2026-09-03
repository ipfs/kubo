package cli

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	shardFuncDefault = "/repo/flatfs/shard/v1/next-to-last/2"
	shardFuncLarge   = "/repo/flatfs/shard/v1/next-to-last/3"
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

// The datastore layout is fixed at init; applying a profile that would change
// it must fail and leave the config untouched, while profiles that keep the
// on-disk layout still apply.
func TestDatastoreProfileApplyAfterInit(t *testing.T) {
	t.Parallel()

	for _, profile := range []string{"flatfs-pebbleds", "pebbleds"} {
		t.Run(profile+" is refused", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init()
			before := node.ReadFile("config")

			res := node.RunIPFS("config", "profile", "apply", profile)
			require.NotEqual(t, 0, res.ExitCode())
			assert.Contains(t, res.Stderr.String(), "fixed when the repo is created")
			assert.Contains(t, res.Stderr.String(), "ipfs init --profile="+profile)
			assert.Equal(t, before, node.ReadFile("config"))

			res = node.RunIPFS("config", "profile", "apply", "--dry-run", profile)
			require.NotEqual(t, 0, res.ExitCode())
			node.IPFS("repo", "stat")
		})
	}

	t.Run("measure wrapper still applies", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.IPFS("config", "profile", "apply", "flatfs-levelds-measure")
		assert.Equal(t, "measure", mountAt(t, node.ReadConfig().Datastore.Spec, "/blocks")["type"])
		node.IPFS("repo", "stat")
	})
}

// Editing shardFunc in the config of an existing repo must fail with an error
// that names the config value, then the on-disk value, and says the layout is
// fixed at init.
func TestDatastoreSpecMismatchError(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init()
	cfg := node.ReadFile("config")
	require.Contains(t, cfg, shardFuncDefault)
	node.WriteBytes("config", []byte(strings.Replace(cfg, shardFuncDefault, shardFuncLarge, 1)))

	res := node.RunIPFS("repo", "stat")
	require.NotEqual(t, 0, res.ExitCode())
	assert.Regexp(t,
		"config Datastore.Spec [^;]*"+regexp.QuoteMeta(shardFuncLarge)+
			"[^;]* does not match the repo's datastore_spec file [^;]*"+regexp.QuoteMeta(shardFuncDefault),
		res.Stderr.String())
	assert.Contains(t, res.Stderr.String(), "fixed when the repo is created")
}

// Any shardFunc can be set by passing a config file to ipfs init; this is the
// documented route for values the profiles do not cover.
func TestInitConfigFileShardFunc(t *testing.T) {
	t.Parallel()
	nodes := harness.NewT(t).NewNodes(2)
	nodes[0].Init()
	cfg := nodes[0].ReadFile("config")
	require.Contains(t, cfg, shardFuncDefault)
	path := filepath.Join(nodes[1].Dir, "init-config.json")
	require.NoError(t, os.WriteFile(path, []byte(strings.Replace(cfg, shardFuncDefault, shardFuncLarge, 1)), 0o600))

	nodes[1].IPFS("init", path)
	assert.Equal(t, shardFuncLarge, strings.TrimSpace(nodes[1].ReadFile(filepath.Join("blocks", "SHARDING"))))
	assert.Contains(t, nodes[1].ReadFile("datastore_spec"), shardFuncLarge)
	assert.Equal(t, nodes[0].PeerID(), nodes[1].PeerID(), "identity comes from the file")

	// blocks land in three-character shard directories and read back
	nodes[1].StartDaemon("--offline")
	defer nodes[1].StopDaemon()
	data := make([]byte, 2*1024*1024)
	_, err := rand.Read(data)
	require.NoError(t, err)
	cid := nodes[1].IPFSAdd(strings.NewReader(string(data)), "--raw-leaves")
	assert.Equal(t, string(data), nodes[1].IPFS("cat", cid).Stdout.String())
	entries, err := os.ReadDir(filepath.Join(nodes[1].Dir, "blocks"))
	require.NoError(t, err)
	var shardDirs []string
	for _, e := range entries {
		if e.IsDir() && e.Name()[0] != '.' {
			shardDirs = append(shardDirs, e.Name())
		}
	}
	require.NotEmpty(t, shardDirs)
	for _, d := range shardDirs {
		assert.Len(t, d, 3, "shard dir %q", d)
	}
}

// A bad shardFunc in the init config must fail before anything is written,
// so the directory is still usable for a corrected init.
func TestInitRejectsBadShardFunc(t *testing.T) {
	t.Parallel()
	nodes := harness.NewT(t).NewNodes(2)
	nodes[0].Init()
	cfg := nodes[0].ReadFile("config")
	require.Contains(t, cfg, shardFuncDefault)
	path := filepath.Join(nodes[1].Dir, "init-config.json")
	require.NoError(t, os.WriteFile(path, []byte(strings.Replace(cfg, shardFuncDefault, "/repo/flatfs/shard/v1/bogus/3", 1)), 0o600))

	res := nodes[1].RunIPFS("init", path)
	require.NotEqual(t, 0, res.ExitCode())
	assert.Contains(t, res.Stderr.String(), "invalid Datastore.Spec")
	assert.Contains(t, res.Stderr.String(), "next-to-last")
	assert.NoFileExists(t, filepath.Join(nodes[1].Dir, "config"))

	nodes[1].IPFS("init")
	assert.Equal(t, shardFuncDefault, strings.TrimSpace(nodes[1].ReadFile(filepath.Join("blocks", "SHARDING"))))
}
