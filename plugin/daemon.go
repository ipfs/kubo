package plugin

import (
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
)

// PluginDaemon is an interface for daemon plugins. These plugins will be run on
// the daemon and will be given access to an implementation of the CoreAPI.
type PluginDaemon interface {
	Plugin

	Start(coreiface.CoreAPI) error
	Close() error
}
