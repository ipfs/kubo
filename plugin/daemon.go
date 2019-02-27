package plugin

import (
	coreiface "gx/ipfs/QmVzvYWRABgGEv4iu3M9wivWbZKTW29qsU4VTZ2iZEoExX/interface-go-ipfs-core"
)

// PluginDaemon is an interface for daemon plugins. These plugins will be run on
// the daemon and will be given access to an implementation of the CoreAPI.
type PluginDaemon interface {
	Plugin

	Start(coreiface.CoreAPI) error
	Close() error
}
