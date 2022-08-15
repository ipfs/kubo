package plugin

import (
	"github.com/ipfs/kubo/core"
	"go.uber.org/fx"
)

// PluginFx can be used to customize the fx options passed to the go-ipfs app when it is initialized.
//
// This is invasive and depends on internal details such as the structure of the dependency graph,
// so breaking changes might occur between releases.
// So it's recommended to keep this as simple as possible, and stick to overriding interfaces
// with fx.Replace() or fx.Decorate().
//
// The returned options become the complete array of options passed to fx.
// Generally you'll want to append additional options to NodeInfo.FXOptions and return that.
type PluginFx interface {
	Plugin
	Options(core.FXNodeInfo) ([]fx.Option, error)
}
