//go:build !rados
// +build !rados

package radosds

import (
	"fmt"

	"github.com/ipfs/go-ipfs/plugin"
	"github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
)

// Plugins is exported list of plugins that will be loaded
var (
	Plugins = []plugin.Plugin{
		&radosdsPlugin{},
	}
	ErrNotSupported = fmt.Errorf("rados is not supported in this version.")
)

type radosdsPlugin struct{}

var _ plugin.PluginDatastore = (*radosdsPlugin)(nil)

func (*radosdsPlugin) Name() string {
	return "ds-rados"
}

func (*radosdsPlugin) Version() string {
	return "0.0.0"
}

func (*radosdsPlugin) Init(_ *plugin.Environment) error {
	return ErrNotSupported
}

func (*radosdsPlugin) DatastoreTypeName() string {
	return "radosds"
}

type datastoreConfig struct {
	configPath string
	pool       string
}

func (*radosdsPlugin) DatastoreConfigParser() fsrepo.ConfigFromMap {
	return func(params map[string]interface{}) (fsrepo.DatastoreConfig, error) {
		return &datastoreConfig{}, nil
	}
}

func (c *datastoreConfig) DiskSpec() fsrepo.DiskSpec {
	return map[string]interface{}{
		"type":       "radosds",
		"configPath": c.configPath,
		"pool":       c.pool,
	}
}

func (c *datastoreConfig) Create(path string) (repo.Datastore, error) {
	return nil, ErrNotSupported
}
