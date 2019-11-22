package plugin

import "github.com/ipfs/go-ipfs/core"

// PluginDaemonInternal is an interface for daemon plugins. These plugins will be run on
// the daemon and will be given a direct access to the IpfsNode.
//
// Note: PluginDaemonInternal is considered internal and no guarantee is made concerning
// the stability of its API. If you can, use PluginAPI instead.
type PluginDaemonInternal interface {
	Plugin

	Start(*core.IpfsNode) error
}
