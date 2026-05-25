// Plugin loading with cgo (uses dlopen to load .so plugins at runtime).
//go:build (linux || darwin || freebsd) && cgo && !noplugin

package loader

import (
	"errors"
	"plugin"

	iplugin "github.com/ipfs/kubo/plugin"
)

func init() {
	loadPluginFunc = unixLoadPlugin
}

func unixLoadPlugin(fi string) ([]iplugin.Plugin, error) {
	pl, err := plugin.Open(fi)
	if err != nil {
		return nil, err
	}
	pls, err := pl.Lookup("Plugins")
	if err != nil {
		return nil, err
	}

	typePls, ok := pls.(*[]iplugin.Plugin)
	if !ok {
		return nil, errors.New("filed 'Plugins' didn't contain correct type")
	}

	return *typePls, nil
}
