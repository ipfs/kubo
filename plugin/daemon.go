package plugin

import (
	coreiface "gx/ipfs/QmUM3JbzMPPVpsUvUcfCdmeU2tssrdVPnUn5E6RawFjDLC/interface-go-ipfs-core"
)

// PluginDaemon is an interface for daemon plugins. These plugins will be run on
// the daemon and will be given access to an implementation of the CoreAPI.
type PluginDaemon interface {
	Plugin

	Start(coreiface.CoreAPI) error
	Close() error
}
