package peeridlog

import (
	"fmt"

	core "github.com/ipfs/go-ipfs/core"
	plugin "github.com/ipfs/go-ipfs/plugin"
)

// Plugins is exported list of plugins that will be loaded
var Plugins = []plugin.Plugin{
	&peerIDLogPlugin{},
}

// Log all the PeerIDs we connect to.
type peerIDLogPlugin struct{}

var _ plugin.PluginDaemonInternal = (*peerIDLogPlugin)(nil)

// Name returns the plugin's name, satisfying the plugin.Plugin interface.
func (*peerIDLogPlugin) Name() string {
	return "peeridlog"
}

// Version returns the plugin's version, satisfying the plugin.Plugin interface.
func (*peerIDLogPlugin) Version() string {
	return "0.1.0"
}

// Init initializes plugin, satisfying the plugin.Plugin interface. Put any
// initialization logic here.
func (*peerIDLogPlugin) Init(*plugin.Environment) error {
	return nil
}

func (*peerIDLogPlugin) Start(*core.IpfsNode) error {
	fmt.Println("peerIDLogPlugin HELLO!")
	return nil
}

func (*peerIDLogPlugin) Close() error {
	fmt.Println("peerIDLogPlugin GOODBYE!")
	return nil
}
