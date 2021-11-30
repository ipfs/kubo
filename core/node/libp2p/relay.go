package libp2p

import (
	config "github.com/ipfs/go-ipfs-config"
	"github.com/libp2p/go-libp2p-core/peer"

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
			def := relay.DefaultResources()
			// Real defaults live in go-libp2p.
			// Here we apply any overrides from user config.
			opts.Opts = append(opts.Opts, libp2p.EnableRelayService(relay.WithResources(relay.Resources{
				Limit: &relay.RelayLimit{
					Data:     relayOpts.ConnectionDataLimit.WithDefault(def.Limit.Data),
					Duration: relayOpts.ConnectionDurationLimit.WithDefault(def.Limit.Duration),
				},
				MaxCircuits:            int(relayOpts.MaxCircuits.WithDefault(int64(def.MaxCircuits))),
				BufferSize:             int(relayOpts.BufferSize.WithDefault(int64(def.BufferSize))),
				ReservationTTL:         relayOpts.ReservationTTL.WithDefault(def.ReservationTTL),
				MaxReservations:        int(relayOpts.MaxReservations.WithDefault(int64(def.MaxReservations))),
				MaxReservationsPerIP:   int(relayOpts.MaxReservations.WithDefault(int64(def.MaxReservationsPerIP))),
				MaxReservationsPerPeer: int(relayOpts.MaxReservations.WithDefault(int64(def.MaxReservationsPerPeer))),
				MaxReservationsPerASN:  int(relayOpts.MaxReservations.WithDefault(int64(def.MaxReservationsPerASN))),
			})))
		}
		return
	}
}

func StaticRelays(relays []string) func() (opts Libp2pOpts, err error) {
	return func() (opts Libp2pOpts, err error) {
		staticRelays := make([]peer.AddrInfo, 0, len(relays))
		for _, s := range relays {
			var addr *peer.AddrInfo
			addr, err = peer.AddrInfoFromString(s)
			if err != nil {
				return
			}
			staticRelays = append(staticRelays, *addr)
		}
		if len(staticRelays) > 0 {
			opts.Opts = append(opts.Opts, libp2p.StaticRelays(staticRelays))
		}
		return
	}
}

func AutoRelay(addDefaultRelays bool) func() (opts Libp2pOpts, err error) {
	return func() (opts Libp2pOpts, err error) {
		opts.Opts = append(opts.Opts, libp2p.EnableAutoRelay())
		if addDefaultRelays {
			opts.Opts = append(opts.Opts, libp2p.DefaultStaticRelays())
		}
		return
	}
}

func HolePunching(flag config.Flag, hasRelayClient bool) func() (opts Libp2pOpts, err error) {
	return func() (opts Libp2pOpts, err error) {
		if flag.WithDefault(false) {
			if !hasRelayClient {
				log.Fatal("To enable `Swarm.EnableHolePunching` requires `Swarm.RelayClient.Enabled` to be enabled.")
			}
			opts.Opts = append(opts.Opts, libp2p.EnableHolePunching())
		}
		return
	}
}
