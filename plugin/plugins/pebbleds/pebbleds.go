package pebbleds

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/cockroachdb/pebble"
	pebbleds "github.com/ipfs/go-ds-pebble"
	"github.com/ipfs/kubo/misc/fsutil"
	"github.com/ipfs/kubo/plugin"
	"github.com/ipfs/kubo/repo"
	"github.com/ipfs/kubo/repo/fsrepo"
)

// Plugins is exported list of plugins that will be loaded.
var Plugins = []plugin.Plugin{
	&pebbledsPlugin{},
}

type pebbledsPlugin struct{}

var _ plugin.PluginDatastore = (*pebbledsPlugin)(nil)

func (*pebbledsPlugin) Name() string {
	return "ds-pebble"
}

func (*pebbledsPlugin) Version() string {
	return "0.1.0"
}

func (*pebbledsPlugin) Init(_ *plugin.Environment) error {
	return nil
}

func (*pebbledsPlugin) DatastoreTypeName() string {
	return "pebbleds"
}

type datastoreConfig struct {
	path      string
	cacheSize int64

	// Documentation of these values: https://pkg.go.dev/github.com/cockroachdb/pebble@v1.1.2#Options
	pebbleOpts *pebble.Options
}

// PebbleDatastoreConfig returns a configuration stub for a pebble datastore
// from the given parameters.
func (*pebbledsPlugin) DatastoreConfigParser() fsrepo.ConfigFromMap {
	return func(params map[string]any) (fsrepo.DatastoreConfig, error) {
		var c datastoreConfig
		var ok bool

		c.path, ok = params["path"].(string)
		if !ok {
			return nil, fmt.Errorf("'path' field is missing or not string")
		}

		cacheSize, err := getConfigInt("cacheSize", params)
		if err != nil {
			return nil, err
		}
		c.cacheSize = int64(cacheSize)

		bytesPerSync, err := getConfigInt("bytesPerSync", params)
		if err != nil {
			return nil, err
		}
		disableWAL, err := getConfigBool("disableWAL", params)
		if err != nil {
			return nil, err
		}
		l0CompactionThreshold, err := getConfigInt("l0CompactionThreshold", params)
		if err != nil {
			return nil, err
		}
		l0StopWritesThreshold, err := getConfigInt("l0StopWritesThreshold", params)
		if err != nil {
			return nil, err
		}
		lBaseMaxBytes, err := getConfigInt("lBaseMaxBytes", params)
		if err != nil {
			return nil, err
		}
		maxConcurrentCompactions, err := getConfigInt("maxConcurrentCompactions", params)
		if err != nil {
			return nil, err
		}
		memTableSize, err := getConfigInt("memTableSize", params)
		if err != nil {
			return nil, err
		}
		memTableStopWritesThreshold, err := getConfigInt("memTableStopWritesThreshold", params)
		if err != nil {
			return nil, err
		}
		walBytesPerSync, err := getConfigInt("walBytesPerSync", params)
		if err != nil {
			return nil, err
		}
		walMinSyncSec, err := getConfigInt("walMinSyncIntervalSeconds", params)
		if err != nil {
			return nil, err
		}

		if bytesPerSync != 0 || disableWAL || l0CompactionThreshold != 0 || l0StopWritesThreshold != 0 || lBaseMaxBytes != 0 || maxConcurrentCompactions != 0 || memTableSize != 0 || memTableStopWritesThreshold != 0 || walBytesPerSync != 0 || walMinSyncSec != 0 {
			c.pebbleOpts = &pebble.Options{
				BytesPerSync:                bytesPerSync,
				DisableWAL:                  disableWAL,
				L0CompactionThreshold:       l0CompactionThreshold,
				L0StopWritesThreshold:       l0StopWritesThreshold,
				LBaseMaxBytes:               int64(lBaseMaxBytes),
				MemTableSize:                uint64(memTableSize),
				MemTableStopWritesThreshold: memTableStopWritesThreshold,
				WALBytesPerSync:             walBytesPerSync,
			}
			if maxConcurrentCompactions != 0 {
				c.pebbleOpts.MaxConcurrentCompactions = func() int { return maxConcurrentCompactions }
			}
			if walMinSyncSec != 0 {
				c.pebbleOpts.WALMinSyncInterval = func() time.Duration { return time.Duration(walMinSyncSec) * time.Second }
			}
		}

		return &c, nil
	}
}

func getConfigBool(name string, params map[string]any) (bool, error) {
	val, ok := params[name]
	if ok {
		bval, ok := val.(bool)
		if !ok {
			return false, fmt.Errorf("%q field was not a bool", name)
		}
		return bval, nil
	}
	return false, nil
}

func getConfigInt(name string, params map[string]any) (int, error) {
	val, ok := params[name]
	if ok {
		// TODO: see why val may be an int or a float64.
		ival, ok := val.(int)
		if !ok {
			fval, ok := val.(float64)
			if !ok {
				return 0, fmt.Errorf("%q field was not an integer or a float64", name)
			}
			return int(fval), nil
		}
		return ival, nil
	}
	return 0, nil
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

	if err := fsutil.DirWritable(p); err != nil {
		return nil, err
	}

	return pebbleds.NewDatastore(p, pebbleds.WithCacheSize(c.cacheSize), pebbleds.WithPebbleOpts(c.pebbleOpts))
}
