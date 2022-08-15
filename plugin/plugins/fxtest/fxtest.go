package fxtest

import (
	"os"

	logging "github.com/ipfs/go-log"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/plugin"
	"go.uber.org/fx"
)

var log = logging.Logger("fxtestplugin")

var Plugins = []plugin.Plugin{
	&fxtestPlugin{},
}

// fxtestPlugin is used for testing the fx plugin.
// It merely adds an fx option that logs a debug statement, so we can verify that it works in tests.
type fxtestPlugin struct{}

var _ plugin.PluginFx = (*fxtestPlugin)(nil)

func (p *fxtestPlugin) Name() string {
	return "fx-test"
}

func (p *fxtestPlugin) Version() string {
	return "0.1.0"
}

func (p *fxtestPlugin) Init(env *plugin.Environment) error {
	return nil
}

func (p *fxtestPlugin) Options(info core.FXNodeInfo) ([]fx.Option, error) {
	opts := info.FXOptions
	if os.Getenv("TEST_FX_PLUGIN") != "" {
		opts = append(opts, fx.Invoke(func() {
			log.Debug("invoked test fx function")
		}))
	}
	return opts, nil
}
