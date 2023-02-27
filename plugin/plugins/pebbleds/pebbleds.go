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
	defopts.MemTableSize = 2 << 30           // 2GiB memtable
	defopts.BytesPerSync = 100 * 1024 * 1024 // 100 MiB
	defopts.Levels[0].Compression = pebble.NoCompression
	defopts.Levels[0].FilterPolicy = bloom.FilterPolicy(10)

	return pebbleds.NewDatastore(p, &defopts)
}
