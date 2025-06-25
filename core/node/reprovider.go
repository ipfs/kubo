package node

import (
	"context"
	"time"

	provider "github.com/ipfs/boxo/provider"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/repo"
	"github.com/libp2p/go-libp2p-kad-dht/amino"
	"github.com/libp2p/go-libp2p-kad-dht/dual"
	dreprovider "github.com/libp2p/go-libp2p-kad-dht/dual/reprovider"
	"github.com/libp2p/go-libp2p-kad-dht/fullrt"
	"github.com/libp2p/go-libp2p-kad-dht/reprovider"
	rds "github.com/libp2p/go-libp2p-kad-dht/reprovider/datastore"
	"github.com/libp2p/go-libp2p/core/routing"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	"go.uber.org/fx"
)

type NoopReprovider struct{}

var _ reprovider.Reprovider = &NoopReprovider{}

func (r *NoopReprovider) StartProviding(...mh.Multihash)                        {}
func (r *NoopReprovider) StopProviding(...mh.Multihash)                         {}
func (r *NoopReprovider) InstantProvide(context.Context, ...mh.Multihash) error { return nil }
func (r *NoopReprovider) ForceProvide(context.Context, ...mh.Multihash) error   { return nil }

func Reprovider(reprovide bool, cfg *config.Config) fx.Option {
	if !reprovide || cfg.Reprovider.Sweep.Enabled.WithDefault(config.DefaultReproviderSweepEnabled) {
		return fx.Options(
			fx.Provide(func() reprovider.Reprovider {
				return &NoopReprovider{}
			}))
	}

	mhStore := fx.Provide(func(keyProvider provider.KeyChanFunc, repo repo.Repo) (*rds.MHStore, error) {
		mhStore, err := rds.NewMHStore(context.Background(), repo.Datastore(),
			rds.WithPrefixLen(10),
			rds.WithDatastorePrefix("/reprovider/mhs"),
			rds.WithGCInterval(cfg.Reprovider.Interval.WithDefault(config.DefaultReproviderInterval)),
			rds.WithGCBatchSize(int(cfg.Reprovider.Sweep.MHStoreBatchSize.WithDefault(config.DefaultReproviderSweepMHStoreBatchSize))),
			rds.WithGCFunc(keyProvider),
		)
		if err != nil {
			return nil, err
		}
		keysChan, err := keyProvider(context.Background())
		if err != nil {
			return nil, err
		}
		err = mhStore.Reset(context.Background(), keysChan)
		if err != nil {
			return nil, err
		}
		return mhStore, nil
	})

	type input struct {
		fx.In
		DHT     routing.Routing `name:"dhtc"`
		MHStore *rds.MHStore
	}
	sweepingReprovider := fx.Provide(func(in input) (reprovider.Reprovider, error) {
		switch dht := in.DHT.(type) {
		case *dual.DHT:
			if dht != nil {
				return dreprovider.NewSweepingReprovider(dht,
					dreprovider.WithMHStore(in.MHStore),

					dreprovider.WithReprovideInterval(cfg.Reprovider.Interval.WithDefault(config.DefaultReproviderInterval)),
					dreprovider.WithMaxReprovideDelay(time.Hour),

					dreprovider.WithMaxWorkers(int(cfg.Reprovider.Sweep.MaxWorkers.WithDefault(config.DefaultReproviderSweepMaxWorkers))),
					dreprovider.WithDedicatedPeriodicWorkers(int(cfg.Reprovider.Sweep.DedicatedPeriodicWorkers.WithDefault(config.DefaultReproviderSweepDedicatedPeriodicWorkers))),
					dreprovider.WithDedicatedPeriodicWorkers(int(cfg.Reprovider.Sweep.DedicatedBurstWorkers.WithDefault(config.DefaultReproviderSweepDedicatedBurstWorkers))),
					dreprovider.WithMaxProvideConnsPerWorker(int(cfg.Reprovider.Sweep.MaxProvideConnsPerWorker.WithDefault(config.DefaultReproviderSweepMaxProvideConnsPerWorker))),
				)
			}
		case *fullrt.FullRT:
			if dht != nil {
				return reprovider.NewReprovider(context.Background(),
					reprovider.WithMHStore(in.MHStore),

					reprovider.WithRouter(dht),
					reprovider.WithMessageSender(dht.MessageSender()),
					reprovider.WithPeerID(dht.Host().ID()),
					reprovider.WithSelfAddrs(func() []ma.Multiaddr { return dht.Host().Addrs() }),
					reprovider.WithAddLocalRecord(func(h mh.Multihash) error {
						return dht.Provide(context.Background(), cid.NewCidV1(cid.Raw, h), false)
					}),

					reprovider.WithReplicationFactor(amino.DefaultBucketSize),
					reprovider.WithReprovideInterval(cfg.Reprovider.Interval.WithDefault(config.DefaultReproviderInterval)),
					reprovider.WithMaxReprovideDelay(time.Hour),
					reprovider.WithConnectivityCheckOnlineInterval(1*time.Minute),
					reprovider.WithConnectivityCheckOfflineInterval(5*time.Minute),

					reprovider.WithMaxWorkers(int(cfg.Reprovider.Sweep.MaxWorkers.WithDefault(config.DefaultReproviderSweepMaxWorkers))),
					reprovider.WithDedicatedPeriodicWorkers(int(cfg.Reprovider.Sweep.DedicatedPeriodicWorkers.WithDefault(config.DefaultReproviderSweepDedicatedPeriodicWorkers))),
					reprovider.WithDedicatedPeriodicWorkers(int(cfg.Reprovider.Sweep.DedicatedBurstWorkers.WithDefault(config.DefaultReproviderSweepDedicatedBurstWorkers))),
					reprovider.WithMaxProvideConnsPerWorker(int(cfg.Reprovider.Sweep.MaxProvideConnsPerWorker.WithDefault(config.DefaultReproviderSweepMaxProvideConnsPerWorker))),
				)
			}
		}
		return &NoopReprovider{}, nil
	})

	closeMHStore := fx.Invoke(func(lc fx.Lifecycle, mhStore *rds.MHStore) {
		lc.Append(fx.Hook{
			OnStop: func(_ context.Context) error {
				return mhStore.Close()
			},
		})
	})

	return fx.Options(
		mhStore,
		sweepingReprovider,
		closeMHStore,
	)
}
