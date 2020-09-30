package badger2ds

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ipfs/go-ipfs/plugin"
	"github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/repo/fsrepo"

	badgeropts "github.com/dgraph-io/badger/v2/options"
	humanize "github.com/dustin/go-humanize"
	badger2ds "github.com/ipfs/go-ds-badger2"
)

// Plugins is exported list of plugins that will be loaded
var Plugins = []plugin.Plugin{
	&badger2dsPlugin{},
}

type badger2dsPlugin struct{}

var _ plugin.PluginDatastore = (*badger2dsPlugin)(nil)

func (*badger2dsPlugin) Name() string {
	return "ds-badger2ds"
}

func (*badger2dsPlugin) Version() string {
	return "0.1.0"
}

func (*badger2dsPlugin) Init(_ *plugin.Environment) error {
	return nil
}

func (*badger2dsPlugin) DatastoreTypeName() string {
	return "badger2ds"
}

type datastoreConfig struct {
	path       string
	syncWrites bool
	truncate   bool

	compression          badgeropts.CompressionType
	zstdCompressionLevel int

	blockCacheSize int64
	vlogFileSize   int64
}

// DatastoreConfigParser returns a function that creates a new badger2
// datastore config from a map of badger2 configuration parameters.
func (*badger2dsPlugin) DatastoreConfigParser() fsrepo.ConfigFromMap {
	return func(params map[string]interface{}) (fsrepo.DatastoreConfig, error) {
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

		compression, ok := params["compression"]
		if !ok {
			// If not specified, use go-ds-badger2 defaults
			c.compression = badger2ds.DefaultOptions.Compression
			c.zstdCompressionLevel = badger2ds.DefaultOptions.ZSTDCompressionLevel
		} else {
			if compression, ok := compression.(string); ok {
				switch compression {
				case "none":
					c.compression = badgeropts.None
				case "snappy":
					c.compression = badgeropts.Snappy
				case "zstd1":
					c.compression = badgeropts.ZSTD
					c.zstdCompressionLevel = 1
				case "zstd2":
					c.compression = badgeropts.ZSTD
					c.zstdCompressionLevel = 2
				case "zstd3":
					c.compression = badgeropts.ZSTD
					c.zstdCompressionLevel = 3
				case "":
					// If empty string, use go-ds-badger2 defaults
					c.compression = badger2ds.DefaultOptions.Compression
					c.zstdCompressionLevel = badger2ds.DefaultOptions.ZSTDCompressionLevel
				default:
					return nil, fmt.Errorf("unrecognized value for compression: %s", compression)
				}
			} else {
				return nil, fmt.Errorf("'compression' field is not string")
			}
		}

		bcs, ok := params["blockCacheSize"]
		if !ok {
			// If not specified, use go-ds-badger2 defaults
			c.blockCacheSize = badger2ds.DefaultOptions.BlockCacheSize
		} else {
			if blockCacheSize, ok := bcs.(string); ok {
				s, err := humanize.ParseBytes(blockCacheSize)
				if err != nil {
					return nil, err
				}
				c.blockCacheSize = int64(s)
			} else {
				return nil, fmt.Errorf("'blockCacheSize' field was not a string")
			}
		}

		vls, ok := params["vlogFileSize"]
		if !ok {
			// If not specified, use go-ds-badger2 defaults
			c.vlogFileSize = badger2ds.DefaultOptions.ValueLogFileSize
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
	return map[string]interface{}{
		"type": "badger2ds",
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

	defopts := badger2ds.DefaultOptions
	defopts.SyncWrites = c.syncWrites
	defopts.Truncate = c.truncate
	defopts.Compression = c.compression
	defopts.ZSTDCompressionLevel = c.zstdCompressionLevel
	defopts.BlockCacheSize = c.blockCacheSize
	defopts.ValueLogFileSize = c.vlogFileSize

	return badger2ds.NewDatastore(p, &defopts)
}
