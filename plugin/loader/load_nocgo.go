// Plugin preloading without cgo (no dlopen, plugins are compiled in).
//go:build (linux || darwin || freebsd) && !cgo && !noplugin

package loader

import (
	"errors"

	iplugin "github.com/ipfs/kubo/plugin"
)

func init() {
	loadPluginFunc = nocgoLoadPlugin
}

func nocgoLoadPlugin(fi string) ([]iplugin.Plugin, error) {
	return nil, errors.New("not built with cgo support")
}
