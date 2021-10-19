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

func RelayService(enable bool, relayOpts config.RelayResources) func() (opts Libp2pOpts, err error) {
	return func() (opts Libp2pOpts, err error) {
		if enable {
			r := relay.DefaultResources()
			if relayOpts.Limit.Data > 0 {
				r.Limit.Data = relayOpts.Limit.Data
			}
			if relayOpts.Limit.Duration > 0 {
				r.Limit.Duration = time.Duration(relayOpts.Limit.Duration)
			}
			if relayOpts.MaxCircuits > 0 {
				r.MaxCircuits = relayOpts.MaxCircuits
			}
			if relayOpts.BufferSize > 0 {
				r.BufferSize = relayOpts.BufferSize
			}
			if relayOpts.ReservationTTL > 0 {
				r.ReservationTTL = time.Duration(relayOpts.ReservationTTL)
			}
			if relayOpts.MaxReservations > 0 {
				r.MaxReservations = relayOpts.MaxReservations
			}
			if relayOpts.MaxReservationsPerIP > 0 {
				r.MaxReservationsPerIP = relayOpts.MaxReservationsPerIP
			}
			if relayOpts.MaxReservationsPerPeer > 0 {
				r.MaxReservationsPerPeer = relayOpts.MaxReservationsPerPeer
			}
			if relayOpts.MaxReservationsPerASN > 0 {
				r.MaxReservationsPerASN = relayOpts.MaxReservationsPerASN
			}
			opts.Opts = append(opts.Opts, libp2p.EnableRelayService(relay.WithResources(r)))
		}
		return
	}
}

var AutoRelay = simpleOpt(libp2p.ChainOptions(libp2p.EnableAutoRelay(), libp2p.DefaultStaticRelays()))
