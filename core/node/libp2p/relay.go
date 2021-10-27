package libp2p

import (
	"time"

	config "github.com/ipfs/go-ipfs-config"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
)

func RelayTransport(enableRelay bool) func() (opts Libp2pOpts, err error) {
	return func() (opts Libp2pOpts, err error) {
		if enableRelay {
			opts.Opts = append(opts.Opts, libp2p.EnableRelay())
		} else {
			opts.Opts = append(opts.Opts, libp2p.DisableRelay())
		}
		return
	}
}

func RelayService(enable bool, relayOpts config.RelayService) func() (opts Libp2pOpts, err error) {
	return func() (opts Libp2pOpts, err error) {
		if enable {
			r := relay.DefaultResources()
			// Real defaults live in go-libp2p.
			// Here we apply any overrides from user config.
			if i := int64(relayOpts.ConnectionDataLimit.WithDefault(0)); i > 0 {
				r.Limit.Data = i
			}
			if i := int(relayOpts.ConnectionDurationLimit.WithDefault(0)); i > 0 {
				r.Limit.Duration = time.Duration(i)
			}
			if i := int(relayOpts.MaxCircuits.WithDefault(0)); i > 0 {
				r.MaxCircuits = i
			}
			if i := int(relayOpts.BufferSize.WithDefault(0)); i > 0 {
				r.BufferSize = i
			}
			if i := int(relayOpts.ReservationTTL.WithDefault(0)); i > 0 {
				r.ReservationTTL = time.Duration(i)
			}
			if i := int(relayOpts.MaxReservations.WithDefault(0)); i > 0 {
				r.MaxReservations = i
			}
			if i := int(relayOpts.MaxReservationsPerIP.WithDefault(0)); i > 0 {
				r.MaxReservationsPerIP = i
			}
			if i := int(relayOpts.MaxReservationsPerPeer.WithDefault(0)); i > 0 {
				r.MaxReservationsPerPeer = i
			}
			if i := int(relayOpts.MaxReservationsPerASN.WithDefault(0)); i > 0 {
				r.MaxReservationsPerASN = i
			}
			opts.Opts = append(opts.Opts, libp2p.EnableRelayService(relay.WithResources(r)))
		}
		return
	}
}

var AutoRelay = simpleOpt(libp2p.ChainOptions(libp2p.EnableAutoRelay(), libp2p.DefaultStaticRelays()))
