package integrationtest

import (
	"github.com/ipfs/go-ipfs/plugin"
	"github.com/ipfs/go-ipfs/plugin/loader"
)

// Initializes a newly created loader with the given plugin objects
// (not plugin names or files).
func loadPlugins(pls ...plugin.Plugin) (*loader.PluginLoader, error) {
	plugins, err := loader.NewPluginLoader("")
	if err != nil {
		return nil, err
	}
	for _, pl := range pls {
		if err := plugins.Load(pl); err != nil {
			return nil, err
		}
	}
	if err := plugins.Initialize(); err != nil {
		return nil, err
	}
	if err := plugins.Inject(); err != nil {
		return nil, err
	}
	return plugins, nil
}
