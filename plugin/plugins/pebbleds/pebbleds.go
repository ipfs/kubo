package pebbleds

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/bloom"
	"github.com/ipfs/kubo/plugin"
	"github.com/ipfs/kubo/repo"
	"github.com/ipfs/kubo/repo/fsrepo"

	pebbleds "github.com/ipfs/go-ds-pebble"
)

// Plugins is exported list of plugins that will be loaded
var Plugins = []plugin.Plugin{
	&pebbledsPlugin{},
}

type pebbledsPlugin struct{}

var _ plugin.PluginDatastore = (*pebbledsPlugin)(nil)

func (*pebbledsPlugin) Name() string {
	return "ds-pebbleds"
}

func (*pebbledsPlugin) Version() string {
	return "0.0.1"
}

func (*pebbledsPlugin) Init(_ *plugin.Environment) error {
	return nil
}

func (*pebbledsPlugin) DatastoreTypeName() string {
	return "pebbleds"
}

type datastoreConfig struct {
	path string
}

// PebbledsDatastoreConfig returns a configuration stub for a pebble datastore
// from the given parameters
func (*pebbledsPlugin) DatastoreConfigParser() fsrepo.ConfigFromMap {
	return func(params map[string]interface{}) (fsrepo.DatastoreConfig, error) {
		var c datastoreConfig
		var ok bool

		c.path, ok = params["path"].(string)
		if !ok {
			return nil, fmt.Errorf("'path' field is missing or not string")
		}

		return &c, nil
	}
}

func (c *datastoreConfig) DiskSpec() fsrepo.DiskSpec {
	return map[string]interface{}{
		"type": "pebbleds",
		"path": c.path,
	}
}

func (c *datastoreConfig) Create(path string) (repo.Datastore, error) {
	p := c.path

	if !filepath.IsAbs(p) {
		p = filepath.Join(path, p)
	}

	err := os.MkdirAll(p, 0755)
	if err != nil {
		return nil, err
	}

	var defopts pebble.Options
	defopts = *defopts.EnsureDefaults()
	defopts.MemTableSize = 4 * 1023 * 1024 * 1024            // Almost 4 GiB
	defopts.BytesPerSync = 500 * 1024 * 1024                 // 500 MiB
	defopts.Cache = pebble.NewCache(10 * 1024 * 1024 * 1024) // 10 GiB
	defopts.DisableWAL = true
	// See https://github.com/cockroachdb/cockroach/blob/a3039fe628f2ab7c5fba31a30ba7bc7c38065230/pkg/storage/pebble.go#L483
	defopts.MaxConcurrentCompactions = func() int {
		return 3
	}
	defopts.L0CompactionThreshold = 2
	defopts.L0StopWritesThreshold = 1000
	defopts.Levels = make([]pebble.LevelOptions, 7)
	defopts.MemTableStopWritesThreshold = 4

	for i := 0; i < len(defopts.Levels); i++ {
		l := &defopts.Levels[i]
		l.BlockSize = 64 << 10       // 64 KB
		l.IndexBlockSize = 512 << 10 // 256 KB
		l.FilterPolicy = bloom.FilterPolicy(10)
		l.FilterType = pebble.TableFilter
		l.Compression = pebble.NoCompression
		if i > 0 {
			l.TargetFileSize = defopts.Levels[i-1].TargetFileSize * 2
		}
		l.EnsureDefaults()
	}

	return pebbleds.NewDatastore(p, &defopts)
}
