package libp2p

import (
	"context"
	"io"
	"sort"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	record "github.com/libp2p/go-libp2p-record"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	routedhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	"go.uber.org/fx"

	config "github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/node/helpers"
	"github.com/ipfs/kubo/repo"
	irouting "github.com/ipfs/kubo/routing"
)

type P2PHostIn struct {
	fx.In

	Repo       repo.Repo
	Validator  record.Validator
	HostOption HostOption
	ID         peer.ID
	Peerstore  peerstore.Peerstore

	Opts [][]libp2p.Option `group:"libp2p"`
}

type P2PHostOut struct {
	fx.Out

	Host    host.Host
	Routers []Router `group:"routers,flatten"`
}

func HostAndRouters(routers map[string]config.Router, experimentalTrackFullNetworkDHT bool) interface{} {
	return func(mctx helpers.MetricsCtx, lc fx.Lifecycle, params P2PHostIn) (out P2PHostOut, err error) {
		opts := []libp2p.Option{libp2p.NoListenAddrs}
		for _, o := range params.Opts {
			opts = append(opts, o...)
		}

		ctx := helpers.LifecycleCtx(mctx, lc)

		cfg, err := params.Repo.Config()
		if err != nil {
			return out, err
		}
		bootstrappers, err := cfg.BootstrapPeers()
		if err != nil {
			return out, err
		}

		opts = append(opts, libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			var rOut []Router
			for _, r := range routers {
				var err error
				var rr routing.Routing
				switch config.RouterType(r.Type) {
				case config.RouterTypeReframe:
					rr, err = irouting.ReframeRoutingFromConfig(r)
					if err != nil {
						return nil, err
					}
				case config.RouterTypeDHT:
					rr, err = irouting.DHTRoutingFromConfig(r, &irouting.ExtraDHTParams{
						ExperimentalTrackFullNetworkDHT: experimentalTrackFullNetworkDHT,
						BootstrapPeers:                  bootstrappers,
						Host:                            h,
						Validator:                       params.Validator,
						Datastore:                       params.Repo.Datastore(),
						Context:                         ctx,
					})
				case config.RouterTypeNone:
					rr = routinghelpers.Null{}
				default:
					err = &irouting.RouterTypeNotFoundError{
						RouterType: r.Type,
					}
				}

				if err != nil {
					return nil, err
				}

				lc.Append(fx.Hook{
					OnStop: func(ctx context.Context) error {
						rc, ok := rr.(io.Closer)
						if !ok {
							return nil
						}

						return rc.Close()
					},
				})

				rOut = append(rOut, Router{
					Routing:  rr,
					Priority: irouting.GetPriority(r.Parameters),
				})
			}

			sort.SliceStable(rOut, func(i, j int) bool {
				return rOut[i].Priority < rOut[j].Priority
			})

			irouters := make([]routing.Routing, len(routers))
			for i, v := range rOut {
				irouters[i] = v.Routing
			}
			r := irouting.Tiered{
				Tiered: routinghelpers.Tiered{
					Routers:   irouters,
					Validator: params.Validator,
				},
			}

			out.Routers = rOut
			return r, nil
		}))

		out.Host, err = params.HostOption(params.ID, params.Peerstore, opts...)
		if err != nil {
			return P2PHostOut{}, err
		}

		// this code is necessary just for tests: mock network constructions
		// ignore the libp2p constructor options that actually construct the routing!
		if len(out.Routers) == 0 {
			r, err := irouting.CreateDHT(&irouting.ExtraDHTParams{
				ExperimentalTrackFullNetworkDHT: false,
				BootstrapPeers:                  bootstrappers,
				Host:                            out.Host,
				Validator:                       params.Validator,
				Datastore:                       params.Repo.Datastore(),
				Context:                         ctx,
			}, true, dht.ModeAuto)
			if err != nil {
				return P2PHostOut{}, err
			}

			tr := irouting.Tiered{
				Tiered: routinghelpers.Tiered{
					Routers:   []routing.Routing{r},
					Validator: params.Validator,
				},
			}

			out.Routers = []Router{{Routing: r, Priority: 1}}
			out.Host = routedhost.Wrap(out.Host, tr)
		}

		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return out.Host.Close()
			},
		})

		return out, err
	}
}
