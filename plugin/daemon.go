package plugin

import (
	coreiface "gx/ipfs/QmNuVxikLRanHXwpw3sy58Vt5yMd4K5BRmWsUqYCctRRVE/interface-go-ipfs-core"
)

// PluginDaemon is an interface for daemon plugins. These plugins will be run on
// the daemon and will be given access to an implementation of the CoreAPI.
type PluginDaemon interface {
	Plugin

	Start(coreiface.CoreAPI) error
	Close() error
}
