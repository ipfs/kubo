package node

import (
	"context"
	"time"

	provider "github.com/ipfs/boxo/provider"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/repo"
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

func Reprovider(cfg *config.Config) fx.Option {
	mhStore := fx.Provide(func(keyProvider provider.KeyChanFunc, repo repo.Repo) (*rds.MHStore, error) {
		return rds.NewMHStore(context.Background(), repo.Datastore(),
			rds.WithPrefixLen(10),
			rds.WithDatastorePrefix("/reprovider/mhs"),
			rds.WithGCInterval(22*time.Hour),
			rds.WithGCBatchSize(1<<14), // ~544 KiB per batch (1 multihash = 34 bytes)
			rds.WithGCFunc(keyProvider),
		)
	})

	type input struct {
		fx.In
		DHT     routing.Routing `name:"dhtc"`
		MHStore *rds.MHStore
	}
	sweepingReprovider := fx.Provide(func(in input) (reprovider.Reprovider, error) {
		reprovideInterval := 22 * time.Hour
		maxReprovideDelay := 1 * time.Hour

		switch dht := in.DHT.(type) {
		case *dual.DHT:
			if dht != nil {
				return dreprovider.NewSweepingReprovider(dht,
					dreprovider.WithMHStore(in.MHStore),

					dreprovider.WithReprovideInterval(reprovideInterval),
					dreprovider.WithMaxReprovideDelay(maxReprovideDelay),

					dreprovider.WithMaxWorkers(4),
					dreprovider.WithDedicatedPeriodicWorkers(2),
					dreprovider.WithDedicatedPeriodicWorkers(1),
					dreprovider.WithMaxProvideConnsPerWorker(20),
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

					reprovider.WithReplicationFactor(20),
					reprovider.WithReprovideInterval(reprovideInterval),
					reprovider.WithMaxReprovideDelay(maxReprovideDelay),
					reprovider.WithConnectivityCheckOnlineInterval(1*time.Minute),
					reprovider.WithConnectivityCheckOfflineInterval(5*time.Minute),

					reprovider.WithMaxWorkers(4),
					reprovider.WithDedicatedPeriodicWorkers(2),
					reprovider.WithDedicatedPeriodicWorkers(1),
					reprovider.WithMaxProvideConnsPerWorker(20),
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
