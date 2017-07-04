package loader

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"plugin"

	iplugin "github.com/ipfs/go-ipfs/plugin"
)

func init() {
	loadPluginsFunc = linxuLoadFunc
}

func linxuLoadFunc(pluginDir string) ([]iplugin.Plugin, error) {
	var plugins []iplugin.Plugin

	filepath.Walk(pluginDir, func(fi string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			log.Warningf("found directory inside plugins directory: %s", fi)
			return nil
		}

		if info.Mode().Perm()&0111 == 0 {
			// file is not executable let's not load it
			// this is to prevent loading plugins from for example non-executable
			// mounts, some /tmp mounts are marked as such for security
			log.Warningf("non-executable file in plugins directory: %s", fi)
			return nil
		}

		if newPlugins, err := loadPlugin(fi); err == nil {
			plugins = append(plugins, newPlugins...)
		} else {
			return fmt.Errorf("loading plugin %s: %s", fi, err)
		}
		return nil
	})

	return plugins, nil
}

func loadPlugin(fi string) ([]iplugin.Plugin, error) {
	pl, err := plugin.Open(fi)
	if err != nil {
		return nil, err
	}
	pls, err := pl.Lookup("Plugins")
	if err != nil {
		return nil, err
	}

	typePls, ok := pls.([]iplugin.Plugin)
	if !ok {
		return nil, errors.New("filed 'Plugins' didn't contain correct type")
	}

	return typePls, nil
}
