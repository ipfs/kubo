//go:build noplugin

package loader

import (
	"errors"

	iplugin "github.com/ipfs/kubo/plugin"
)

func init() {
	loadPluginFunc = nopluginLoadPlugin
}

func nopluginLoadPlugin(string) ([]iplugin.Plugin, error) {
	return nil, errors.New("not built with plugin support")
}
