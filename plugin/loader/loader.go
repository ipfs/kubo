package loader

import (
	"fmt"
	"os"
	"strings"

	coredag "github.com/ipfs/go-ipfs/core/coredag"
	plugin "github.com/ipfs/go-ipfs/plugin"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"

	opentracing "gx/ipfs/QmWLWmRVSiagqP15jczsGME1qpob6HDbtbHAY2he9W5iUo/opentracing-go"
	coreiface "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core"
	ipld "gx/ipfs/QmZ6nzCLwGLVfRzYLpD7pW6UNuBDKEcA2imJtVpbEx2rxy/go-ipld-format"
	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"
)

var log = logging.Logger("plugin/loader")

var loadPluginsFunc = func(string) ([]plugin.Plugin, error) {
	return nil, nil
}

// PluginLoader keeps track of loaded plugins
type PluginLoader struct {
	plugins []plugin.Plugin
}

// NewPluginLoader creates new plugin loader
func NewPluginLoader(pluginDir string) (*PluginLoader, error) {
	plMap := make(map[string]plugin.Plugin)
	for _, v := range preloadPlugins {
		plMap[v.Name()] = v
	}

	if pluginDir != "" {
		newPls, err := loadDynamicPlugins(pluginDir)
		if err != nil {
			return nil, err
		}

		for _, pl := range newPls {
			if ppl, ok := plMap[pl.Name()]; ok {
				// plugin is already preloaded
				return nil, fmt.Errorf(
					"plugin: %s, is duplicated in version: %s, "+
						"while trying to load dynamically: %s",
					ppl.Name(), ppl.Version(), pl.Version())
			}
			plMap[pl.Name()] = pl
		}
	}

	loader := &PluginLoader{plugins: make([]plugin.Plugin, 0, len(plMap))}

	for _, v := range plMap {
		loader.plugins = append(loader.plugins, v)
	}

	return loader, nil
}

func loadDynamicPlugins(pluginDir string) ([]plugin.Plugin, error) {
	_, err := os.Stat(pluginDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return loadPluginsFunc(pluginDir)
}

// Initialize initializes all loaded plugins
func (loader *PluginLoader) Initialize() error {
	for _, p := range loader.plugins {
		err := p.Init()
		if err != nil {
			return err
		}
	}

	return nil
}

// Inject hooks all the plugins into the appropriate subsystems.
func (loader *PluginLoader) Inject() error {
	for _, pl := range loader.plugins {
		if pl, ok := pl.(plugin.PluginIPLD); ok {
			err := injectIPLDPlugin(pl)
			if err != nil {
				return err
			}
		}
		if pl, ok := pl.(plugin.PluginTracer); ok {
			err := injectTracerPlugin(pl)
			if err != nil {
				return err
			}
		}
		if pl, ok := pl.(plugin.PluginDatastore); ok {
			err := injectDatastorePlugin(pl)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Start starts all long-running plugins.
func (loader *PluginLoader) Start(iface coreiface.CoreAPI) error {
	for i, pl := range loader.plugins {
		if pl, ok := pl.(plugin.PluginDaemon); ok {
			err := pl.Start(iface)
			if err != nil {
				closePlugins(loader.plugins[i:])
				return err
			}
		}
	}
	return nil
}

// StopDaemon stops all long-running plugins.
func (loader *PluginLoader) Close() error {
	return closePlugins(loader.plugins)
}

func closePlugins(plugins []plugin.Plugin) error {
	var errs []string
	for _, pl := range plugins {
		if pl, ok := pl.(plugin.PluginDaemon); ok {
			err := pl.Close()
			if err != nil {
				errs = append(errs, fmt.Sprintf(
					"error closing plugin %s: %s",
					pl.Name(),
					err.Error(),
				))
			}
		}
	}
	if errs != nil {
		return fmt.Errorf(strings.Join(errs, "\n"))
	}
	return nil
}

func injectDatastorePlugin(pl plugin.PluginDatastore) error {
	return fsrepo.AddDatastoreConfigHandler(pl.DatastoreTypeName(), pl.DatastoreConfigParser())
}

func injectIPLDPlugin(pl plugin.PluginIPLD) error {
	err := pl.RegisterBlockDecoders(ipld.DefaultBlockDecoder)
	if err != nil {
		return err
	}
	return pl.RegisterInputEncParsers(coredag.DefaultInputEncParsers)
}

func injectTracerPlugin(pl plugin.PluginTracer) error {
	tracer, err := pl.InitTracer()
	if err != nil {
		return err
	}
	opentracing.SetGlobalTracer(tracer)
	return nil
}
