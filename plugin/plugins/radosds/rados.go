//go:build rados
// +build rados

package radosds

import (
	"fmt"

	"github.com/ipfs/go-ipfs/plugin"
	"github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/repo/fsrepo"

	rados "github.com/coryschwartz/go-ds-rados"
)

// Plugins is exported list of plugins that will be loaded
var Plugins = []plugin.Plugin{
	&radosdsPlugin{},
}

type radosdsPlugin struct{}

var _ plugin.PluginDatastore = (*radosdsPlugin)(nil)

func (*radosdsPlugin) Name() string {
	return "ds-rados"
}

func (*radosdsPlugin) Version() string {
	return "0.0.0"
}

func (*radosdsPlugin) Init(_ *plugin.Environment) error {
	fmt.Println("radosds init")
	return nil
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
		var c datastoreConfig
		var ok bool

		// if ConfigPath is not set, the default ceph config
		// will be used. It's okay if it isn't set.
		c.configPath, ok = params["configPath"].(string)
		if !ok {
			c.configPath = ""
		}
		c.pool, ok = params["pool"].(string)
		if !ok {
			c.pool = ""
		}
		return &c, nil
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
	fmt.Println("radosds create")
	return rados.NewDatastore(c.configPath, c.pool)
}
