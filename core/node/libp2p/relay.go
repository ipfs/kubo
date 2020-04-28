package libp2p

import (
	"github.com/libp2p/go-libp2p"
	relay "github.com/libp2p/go-libp2p-circuit"
)

func Relay(disable, enableHop bool) func() (opts Libp2pOpts, err error) {
	return func() (opts Libp2pOpts, err error) {
		if disable {
			// Enabled by default.
			opts.Opts = append(opts.Opts, libp2p.DisableRelay())
		} else {
			relayOpts := []relay.RelayOpt{}
			if enableHop {
				relayOpts = append(relayOpts, relay.OptHop)
			}
			opts.Opts = append(opts.Opts, libp2p.EnableRelay(relayOpts...))
		}
		return
	}
}

var AutoRelay = simpleOpt(libp2p.ChainOptions(libp2p.EnableAutoRelay(), libp2p.DefaultStaticRelays()))
