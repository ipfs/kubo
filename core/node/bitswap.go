package node

import (
	"context"
	"time"

	blockstore "github.com/ipfs/go-ipfs-blockstore"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	"github.com/ipfs/go-libipfs/bitswap"
	"github.com/ipfs/go-libipfs/bitswap/network"
	"github.com/ipfs/kubo/config"
	irouting "github.com/ipfs/kubo/routing"
	"github.com/libp2p/go-libp2p/core/host"
	"go.uber.org/fx"

	"github.com/ipfs/kubo/core/node/helpers"
)

// Docs: https://github.com/ipfs/kubo/blob/master/docs/config.md#internalbitswap
const (
	DefaultEngineBlockstoreWorkerCount = 128
	DefaultTaskWorkerCount             = 8
	DefaultEngineTaskWorkerCount       = 8
	DefaultMaxOutstandingBytesPerPeer  = 1 << 20
	DefaultProviderSearchDelay         = 1000 * time.Millisecond
)

type bitswapOptionsOut struct {
	fx.Out

	BitswapOpts []bitswap.Option `group:"bitswap-options,flatten"`
}

// BitswapOptions creates configuration options for Bitswap from the config file
// and whether to provide data.
func BitswapOptions(cfg *config.Config, provide bool) interface{} {
	return func() bitswapOptionsOut {
		var internalBsCfg config.InternalBitswap
		if cfg.Internal.Bitswap != nil {
			internalBsCfg = *cfg.Internal.Bitswap
		}

		opts := []bitswap.Option{
			bitswap.ProvideEnabled(provide),
			bitswap.ProviderSearchDelay(internalBsCfg.ProviderSearchDelay.WithDefault(DefaultProviderSearchDelay)), // See https://github.com/ipfs/go-ipfs/issues/8807 for rationale
			bitswap.EngineBlockstoreWorkerCount(int(internalBsCfg.EngineBlockstoreWorkerCount.WithDefault(DefaultEngineBlockstoreWorkerCount))),
			bitswap.TaskWorkerCount(int(internalBsCfg.TaskWorkerCount.WithDefault(DefaultTaskWorkerCount))),
			bitswap.EngineTaskWorkerCount(int(internalBsCfg.EngineTaskWorkerCount.WithDefault(DefaultEngineTaskWorkerCount))),
			bitswap.MaxOutstandingBytesPerPeer(int(internalBsCfg.MaxOutstandingBytesPerPeer.WithDefault(DefaultMaxOutstandingBytesPerPeer))),
		}

		return bitswapOptionsOut{BitswapOpts: opts}
	}
}

type onlineExchangeIn struct {
	fx.In

	Mctx        helpers.MetricsCtx
	Host        host.Host
	Rt          irouting.ProvideManyRouter
	Bs          blockstore.GCBlockstore
	BitswapOpts []bitswap.Option `group:"bitswap-options"`
}

// OnlineExchange creates new LibP2P backed block exchange (BitSwap).
// Additional options to bitswap.New can be provided via the "bitswap-options"
// group.
func OnlineExchange() interface{} {
	return func(in onlineExchangeIn, lc fx.Lifecycle) exchange.Interface {
		bitswapNetwork := network.NewFromIpfsHost(in.Host, in.Rt)

		exch := bitswap.New(helpers.LifecycleCtx(in.Mctx, lc), bitswapNetwork, in.Bs, in.BitswapOpts...)
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return exch.Close()
			},
		})
		return exch
	}
}
