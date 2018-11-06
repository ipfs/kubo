package radosds

import (
	"fmt"
	"os"

	"github.com/ipfs/go-ipfs/plugin"
	"github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/repo/fsrepo"

	rados "gx/ipfs/QmdZuk8xgy7o832HfhMEYFpVfWjGFm9FdgRW4ZatK1nwwQ/go-ds-rados"
)

// Plugins is exported list of plugins that will be loaded
var Plugins = []plugin.Plugin{
	&radosPlugin{},
}

type radosPlugin struct{}

var _ plugin.PluginDatastore = (*radosPlugin)(nil)

func (*radosPlugin) Name() string {
	return "ds-rados"
}

func (*radosPlugin) Version() string {
	return "0.1.0"
}

func (*radosPlugin) Init() error {
	return nil
}

func (*radosPlugin) DatastoreTypeName() string {
	return "rados"
}

type radosDatastoreConfig struct {
	confPath string
	pool     string
}

func (*radosPlugin) DatastoreConfigParser() fsrepo.ConfigFromMap {
	return func(params map[string]interface{}) (fsrepo.DatastoreConfig, error) {
		var c radosDatastoreConfig
		var ok bool
		c.confPath, ok = params["confpath"].(string)
		if !ok {
			return nil, fmt.Errorf("confpath filed is missing or not string")
		}
		c.pool, ok = params["pool"].(string)
		if !ok {
			return nil, fmt.Errorf("'pool' filed is missing or not string")
		}
		return &c, nil
	}
}

func (c *radosDatastoreConfig) DiskSpec() fsrepo.DiskSpec {
	return map[string]interface{}{
		"type":     "rados",
		"confpath": c.confPath,
		"pool":     c.pool,
	}
}

func (c *radosDatastoreConfig) Create(string) (repo.Datastore, error) {
	_, err := os.Open(c.confPath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("Config file for rados doesn't exist:%s", c.confPath)
	}
	if os.IsPermission(err) {
		return nil, fmt.Errorf("Permission deny for file:%s", c.confPath)
	}

	return rados.NewDatastore(c.confPath, c.pool)
}
