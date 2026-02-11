package badgerds

import (
	"fmt"
	"os"
	"path/filepath"

	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/kubo/plugin"
	"github.com/ipfs/kubo/repo"
	"github.com/ipfs/kubo/repo/fsrepo"

	humanize "github.com/dustin/go-humanize"
	badgerds "github.com/ipfs/go-ds-badger"
)

var log = logging.Logger("plugin/badgerds")

// Plugins is exported list of plugins that will be loaded.
var Plugins = []plugin.Plugin{
	&badgerdsPlugin{},
}

type badgerdsPlugin struct{}

var _ plugin.PluginDatastore = (*badgerdsPlugin)(nil)

func (*badgerdsPlugin) Name() string {
	return "ds-badgerds"
}

func (*badgerdsPlugin) Version() string {
	return "0.1.0"
}

func (*badgerdsPlugin) Init(_ *plugin.Environment) error {
	return nil
}

func (*badgerdsPlugin) DatastoreTypeName() string {
	return "badgerds"
}

type datastoreConfig struct {
	path       string
	syncWrites bool
	truncate   bool

	vlogFileSize int64
}

// BadgerdsDatastoreConfig returns a configuration stub for a badger datastore
// from the given parameters.
func (*badgerdsPlugin) DatastoreConfigParser() fsrepo.ConfigFromMap {
	return func(params map[string]any) (fsrepo.DatastoreConfig, error) {
		var c datastoreConfig
		var ok bool

		c.path, ok = params["path"].(string)
		if !ok {
			return nil, fmt.Errorf("'path' field is missing or not string")
		}

		sw, ok := params["syncWrites"]
		if !ok {
			c.syncWrites = false
		} else {
			if swb, ok := sw.(bool); ok {
				c.syncWrites = swb
			} else {
				return nil, fmt.Errorf("'syncWrites' field was not a boolean")
			}
		}

		truncate, ok := params["truncate"]
		if !ok {
			c.truncate = true
		} else {
			if truncate, ok := truncate.(bool); ok {
				c.truncate = truncate
			} else {
				return nil, fmt.Errorf("'truncate' field was not a boolean")
			}
		}

		vls, ok := params["vlogFileSize"]
		if !ok {
			// default to 1GiB
			c.vlogFileSize = badgerds.DefaultOptions.ValueLogFileSize
		} else {
			if vlogSize, ok := vls.(string); ok {
				s, err := humanize.ParseBytes(vlogSize)
				if err != nil {
					return nil, err
				}
				c.vlogFileSize = int64(s)
			} else {
				return nil, fmt.Errorf("'vlogFileSize' field was not a string")
			}
		}

		return &c, nil
	}
}

func (c *datastoreConfig) DiskSpec() fsrepo.DiskSpec {
	return map[string]any{
		"type": "badgerds",
		"path": c.path,
	}
}

func (c *datastoreConfig) Create(path string) (repo.Datastore, error) {
	log.Error("badger v1 datastore is deprecated and will be removed later in 2026, migrate to flatfs or experimental pebbleds: https://github.com/ipfs/kubo/issues/11186")
	fmt.Fprintf(os.Stderr, `
╔════════════════════════════════════════════════════════════════════════════╗
║                                                                            ║
║  ERROR: BADGER v1 DATASTORE IS DEPRECATED                                  ║
║                                                                            ║
║  This datastore is based on badger 1.x which has not been maintained       ║
║  by its upstream maintainers for years and has known bugs (startup         ║
║  timeouts, shutdown hangs, file descriptor exhaustion, and more).          ║
║                                                                            ║
║  Badger v1 support will be REMOVED later in 2026.                          ║
║                                                                            ║
║  To migrate:                                                               ║
║    1. Create a new IPFS_PATH with flatfs (or experimental pebbleds         ║
║       if flatfs does not serve your use case):                             ║
║         export IPFS_PATH=/path/to/new/repo                                 ║
║         ipfs init --profile=flatfs                                         ║
║    2. Move pinned data via ipfs dag export/import                          ║
║       or ipfs pin ls -t recursive|add                                      ║
║    3. Decommission the old badger-based node                               ║
║                                                                            ║
║  See https://github.com/ipfs/kubo/blob/master/docs/datastores.md           ║
║      https://github.com/ipfs/kubo/issues/11186                             ║
║                                                                            ║
╚════════════════════════════════════════════════════════════════════════════╝
`)
	p := c.path
	if !filepath.IsAbs(p) {
		p = filepath.Join(path, p)
	}

	err := os.MkdirAll(p, 0o755)
	if err != nil {
		return nil, err
	}

	defopts := badgerds.DefaultOptions
	defopts.SyncWrites = c.syncWrites
	defopts.Truncate = c.truncate
	defopts.ValueLogFileSize = c.vlogFileSize

	return badgerds.NewDatastore(p, &defopts)
}
