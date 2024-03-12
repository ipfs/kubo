package pebbleds

import (
	"fmt"
	"path/filepath"

	"github.com/cockroachdb/pebble"
	"github.com/ipfs/kubo/plugin"
	"github.com/ipfs/kubo/repo"
	"github.com/ipfs/kubo/repo/fsrepo"

	pebbleds "github.com/ipfs/go-ds-pebble"
)

var Plugins = []plugin.Plugin{
	&pebbledsPlugin{},
}

type pebbledsPlugin struct{}

var _ plugin.PluginDatastore = (*pebbledsPlugin)(nil)

func (*pebbledsPlugin) Name() string {
	return "ds-pebble"
}

func (*pebbledsPlugin) Version() string {
	return ""
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
	return pebbleds.NewDatastore(p, &pebble.Options{})
}
