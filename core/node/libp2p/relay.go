package libp2p

import (
	"github.com/ipfs/go-ipfs/config"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
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

func AutoRelay(staticRelays []string, peerChan <-chan peer.AddrInfo) func() (opts Libp2pOpts, err error) {
	return func() (opts Libp2pOpts, err error) {
		var autoRelayOpts []autorelay.Option
		if len(staticRelays) > 0 {
			static := make([]peer.AddrInfo, 0, len(staticRelays))
			for _, s := range staticRelays {
				var addr *peer.AddrInfo
				addr, err = peer.AddrInfoFromString(s)
				if err != nil {
					return
				}
				static = append(static, *addr)
			}
			autoRelayOpts = append(autoRelayOpts, autorelay.WithStaticRelays(static))
			autoRelayOpts = append(autoRelayOpts, autorelay.WithCircuitV1Support())
		}
		if peerChan != nil {
			autoRelayOpts = append(autoRelayOpts, autorelay.WithPeerSource(peerChan))
		}
		opts.Opts = append(opts.Opts, libp2p.EnableAutoRelay(autoRelayOpts...))
		return
	}
}

func HolePunching(flag config.Flag, hasRelayClient bool) func() (opts Libp2pOpts, err error) {
	return func() (opts Libp2pOpts, err error) {
		if flag.WithDefault(true) {
			if !hasRelayClient {
				log.Fatal("Failed to enable `Swarm.EnableHolePunching`, it requires `Swarm.RelayClient.Enabled` to be true.")
				return
			}
			opts.Opts = append(opts.Opts, libp2p.EnableHolePunching())
		}
		return
	}
}
