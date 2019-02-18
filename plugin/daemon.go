package plugin

import (
	coreiface "gx/ipfs/QmNt8iUv3uJoVrJ3Ls4cgMLk124V4Bt1JxuUMWbjULt9ns/interface-go-ipfs-core"
)

// PluginDaemon is an interface for daemon plugins. These plugins will be run on
// the daemon and will be given access to an implementation of the CoreAPI.
type PluginDaemon interface {
	Plugin

	Start(coreiface.CoreAPI) error
	Close() error
}
