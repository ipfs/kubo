package libp2p

import (
	"context"

	"github.com/ipfs/kubo/config"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"go.uber.org/fx"
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
				MaxCircuits:           int(relayOpts.MaxCircuits.WithDefault(int64(def.MaxCircuits))),
				BufferSize:            int(relayOpts.BufferSize.WithDefault(int64(def.BufferSize))),
				ReservationTTL:        relayOpts.ReservationTTL.WithDefault(def.ReservationTTL),
				MaxReservations:       int(relayOpts.MaxReservations.WithDefault(int64(def.MaxReservations))),
				MaxReservationsPerIP:  int(relayOpts.MaxReservationsPerIP.WithDefault(int64(def.MaxReservationsPerIP))),
				MaxReservationsPerASN: int(relayOpts.MaxReservationsPerASN.WithDefault(int64(def.MaxReservationsPerASN))),
			})))
		}
		return
	}
}

func MaybeAutoRelay(staticRelays []string, cfgPeering config.Peering, enabled bool) fx.Option {
	if !enabled {
		return fx.Options()
	}

	if len(staticRelays) > 0 {
		return fx.Provide(func() (opts Libp2pOpts, err error) {
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
				opts.Opts = append(opts.Opts, libp2p.EnableAutoRelayWithStaticRelays(static))
			}
			return
		})
	}

	peerChan := make(chan peer.AddrInfo)
	return fx.Options(
		// Provide AutoRelay option
		fx.Provide(func() (opts Libp2pOpts, err error) {
			opts.Opts = append(opts.Opts,
				libp2p.EnableAutoRelayWithPeerSource(
					func(ctx context.Context, numPeers int) <-chan peer.AddrInfo {
						// TODO(9257): make this code smarter (have a state and actually try to grow the search outward) instead of a long running task just polling our K cluster.
						r := make(chan peer.AddrInfo)
						go func() {
							defer close(r)
							for ; numPeers != 0; numPeers-- {
								select {
								case v, ok := <-peerChan:
									if !ok {
										return
									}
									select {
									case r <- v:
									case <-ctx.Done():
										return
									}
								case <-ctx.Done():
									return
								}
							}
						}()
						return r
					},
					autorelay.WithMinInterval(0),
				))
			return
		}),
		autoRelayFeeder(cfgPeering, peerChan),
	)
}

func HolePunching(flag config.Flag, hasRelayClient bool) func() (opts Libp2pOpts, err error) {
	return func() (opts Libp2pOpts, err error) {
		if flag.WithDefault(true) {
			if !hasRelayClient {
				// If hole punching is explicitly enabled but the relay client is disabled then panic,
				// otherwise just silently disable hole punching
				if flag != config.Default {
					log.Fatal("Failed to enable `Swarm.EnableHolePunching`, it requires `Swarm.RelayClient.Enabled` to be true.")
				} else {
					log.Info("HolePunching has been disabled due to the RelayClient being disabled.")
				}
				return
			}
			opts.Opts = append(opts.Opts, libp2p.EnableHolePunching())
		}
		return
	}
}
