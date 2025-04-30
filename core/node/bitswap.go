package node

import (
	"context"
	"io"
	"time"

	"github.com/ipfs/boxo/bitswap"
	"github.com/ipfs/boxo/bitswap/client"
	bsnet "github.com/ipfs/boxo/bitswap/network/bsnet"
	blockstore "github.com/ipfs/boxo/blockstore"
	exchange "github.com/ipfs/boxo/exchange"
	"github.com/ipfs/boxo/exchange/providing"
	provider "github.com/ipfs/boxo/provider"
	rpqm "github.com/ipfs/boxo/routing/providerquerymanager"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/kubo/config"
	irouting "github.com/ipfs/kubo/routing"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/routing"
	"go.uber.org/fx"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/kubo/core/node/helpers"
)

// Docs: https://github.com/ipfs/kubo/blob/master/docs/config.md#internalbitswap
const (
	DefaultEngineBlockstoreWorkerCount = 128
	DefaultTaskWorkerCount             = 8
	DefaultEngineTaskWorkerCount       = 8
	DefaultMaxOutstandingBytesPerPeer  = 1 << 20
	DefaultProviderSearchDelay         = 1000 * time.Millisecond
	DefaultMaxProviders                = 10 // matching BitswapClientDefaultMaxProviders from https://github.com/ipfs/boxo/blob/v0.29.1/bitswap/internal/defaults/defaults.go#L15
	DefaultWantHaveReplaceSize         = 1024
)

type bitswapOptionsOut struct {
	fx.Out

	BitswapOpts []bitswap.Option `group:"bitswap-options,flatten"`
}

// BitswapOptions creates configuration options for Bitswap from the config file
// and whether to provide data.
func BitswapOptions(cfg *config.Config) interface{} {
	return func() bitswapOptionsOut {
		var internalBsCfg config.InternalBitswap
		if cfg.Internal.Bitswap != nil {
			internalBsCfg = *cfg.Internal.Bitswap
		}

		opts := []bitswap.Option{
			bitswap.ProviderSearchDelay(internalBsCfg.ProviderSearchDelay.WithDefault(DefaultProviderSearchDelay)), // See https://github.com/ipfs/go-ipfs/issues/8807 for rationale
			bitswap.EngineBlockstoreWorkerCount(int(internalBsCfg.EngineBlockstoreWorkerCount.WithDefault(DefaultEngineBlockstoreWorkerCount))),
			bitswap.TaskWorkerCount(int(internalBsCfg.TaskWorkerCount.WithDefault(DefaultTaskWorkerCount))),
			bitswap.EngineTaskWorkerCount(int(internalBsCfg.EngineTaskWorkerCount.WithDefault(DefaultEngineTaskWorkerCount))),
			bitswap.MaxOutstandingBytesPerPeer(int(internalBsCfg.MaxOutstandingBytesPerPeer.WithDefault(DefaultMaxOutstandingBytesPerPeer))),
			bitswap.WithWantHaveReplaceSize(int(internalBsCfg.WantHaveReplaceSize.WithDefault(DefaultWantHaveReplaceSize))),
		}

		return bitswapOptionsOut{BitswapOpts: opts}
	}
}

type bitswapIn struct {
	fx.In

	Mctx        helpers.MetricsCtx
	Cfg         *config.Config
	Host        host.Host
	Rt          irouting.ProvideManyRouter
	Bs          blockstore.GCBlockstore
	BitswapOpts []bitswap.Option `group:"bitswap-options"`
}

// Bitswap creates the BitSwap server/client instance.
// If Bitswap.ServerEnabled is false, the node will act only as a client
// using an empty blockstore to prevent serving blocks to other peers.
func Bitswap(provide bool) interface{} {
	return func(in bitswapIn, lc fx.Lifecycle) (*bitswap.Bitswap, error) {
		bitswapNetwork := bsnet.NewFromIpfsHost(in.Host)
		var blockstoree blockstore.Blockstore = in.Bs
		var provider routing.ContentDiscovery

		if provide {

			var maxProviders int = DefaultMaxProviders
			if in.Cfg.Internal.Bitswap != nil {
				maxProviders = int(in.Cfg.Internal.Bitswap.ProviderSearchMaxResults.WithDefault(DefaultMaxProviders))
			}

			pqm, err := rpqm.New(bitswapNetwork,
				in.Rt,
				rpqm.WithMaxProviders(maxProviders),
				rpqm.WithIgnoreProviders(in.Cfg.Routing.IgnoreProviders...),
			)
			if err != nil {
				return nil, err
			}
			in.BitswapOpts = append(in.BitswapOpts, bitswap.WithClientOption(client.WithDefaultProviderQueryManager(false)))
			in.BitswapOpts = append(in.BitswapOpts, bitswap.WithServerEnabled(true))
			provider = pqm
		} else {
			provider = nil
			// When server is disabled, use an empty blockstore to prevent serving blocks
			blockstoree = blockstore.NewBlockstore(datastore.NewMapDatastore())
			in.BitswapOpts = append(in.BitswapOpts, bitswap.WithServerEnabled(false))
		}

		bs := bitswap.New(helpers.LifecycleCtx(in.Mctx, lc), bitswapNetwork, provider, blockstoree, in.BitswapOpts...)

		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return bs.Close()
			},
		})
		return bs, nil
	}
}

// OnlineExchange creates new LibP2P backed block exchange.
// Returns a no-op exchange if Bitswap is disabled.
func OnlineExchange(isBitswapActive bool) interface{} {
	return func(in *bitswap.Bitswap, lc fx.Lifecycle) exchange.Interface {
		if !isBitswapActive {
			return &noopExchange{closer: in}
		}
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return in.Close()
			},
		})
		return in
	}
}

type providingExchangeIn struct {
	fx.In

	BaseExch exchange.Interface
	Provider provider.System
}

// ProvidingExchange creates a providing.Exchange with the existing exchange
// and the provider.System.
// We cannot do this in OnlineExchange because it causes cycles so this is for
// a decorator.
func ProvidingExchange(provide bool) interface{} {
	return func(in providingExchangeIn, lc fx.Lifecycle) exchange.Interface {
		exch := in.BaseExch
		if provide {
			exch = providing.New(in.BaseExch, in.Provider)
			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					return exch.Close()
				},
			})
		}
		return exch
	}
}

type noopExchange struct {
	closer io.Closer
}

func (e *noopExchange) GetBlock(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	return nil, ipld.ErrNotFound{c}
}

func (e *noopExchange) GetBlocks(ctx context.Context, cids []cid.Cid) (<-chan blocks.Block, error) {
	ch := make(chan blocks.Block)
	close(ch)
	return ch, nil
}

func (e *noopExchange) NotifyNewBlocks(ctx context.Context, blocks ...blocks.Block) error {
	return nil
}

func (e *noopExchange) Close() error {
	return e.closer.Close()
}
