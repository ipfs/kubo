package node

import (
	"context"
	"time"

	"github.com/ipfs/boxo/bitswap"
	"github.com/ipfs/boxo/bitswap/client"
	"github.com/ipfs/boxo/bitswap/network"
	bsnet "github.com/ipfs/boxo/bitswap/network/bsnet"
	"github.com/ipfs/boxo/bitswap/network/httpnet"
	blockstore "github.com/ipfs/boxo/blockstore"
	exchange "github.com/ipfs/boxo/exchange"
	"github.com/ipfs/boxo/exchange/providing"
	provider "github.com/ipfs/boxo/provider"
	rpqm "github.com/ipfs/boxo/routing/providerquerymanager"
	"github.com/ipfs/kubo/config"
	irouting "github.com/ipfs/kubo/routing"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/routing"
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
// Additional options to bitswap.New can be provided via the "bitswap-options"
// group.
func Bitswap(provide bool) interface{} {
	return func(in bitswapIn, lc fx.Lifecycle) (*bitswap.Bitswap, error) {
		var ntwk network.BitSwapNetwork
		bitswapNetwork := bsnet.NewFromIpfsHost(in.Host)

		if httpCfg := in.Cfg.HTTPRetrieval; httpCfg.Enabled {
			htnet := httpnet.New(in.Host,
				httpnet.WithHTTPWorkers(int(httpCfg.NumWorkers.WithDefault(16))),
				httpnet.WithAllowlist(httpCfg.Allowlist),
				httpnet.WithDenylist(httpCfg.Denylist),
			)
			ntwk = network.New(in.Host.Peerstore(), bitswapNetwork, htnet)
		} else {
			ntwk = bitswapNetwork
		}

		var provider routing.ContentDiscovery
		if provide {
			// We need to hardcode the default because it is an
			// internal setting in boxo.
			pqm, err := rpqm.New(ntwk,
				in.Rt,
				rpqm.WithMaxProviders(10),
				rpqm.WithIgnoreProviders(in.Cfg.Routing.IgnoreProviders...),
			)
			if err != nil {
				return nil, err
			}
			in.BitswapOpts = append(in.BitswapOpts, bitswap.WithClientOption(client.WithDefaultProviderQueryManager(false)))
			provider = pqm

		}
		bs := bitswap.New(helpers.LifecycleCtx(in.Mctx, lc), ntwk, provider, in.Bs, in.BitswapOpts...)

		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return bs.Close()
			},
		})
		return bs, nil
	}
}

// OnlineExchange creates new LibP2P backed block exchange.
func OnlineExchange() interface{} {
	return func(in *bitswap.Bitswap, lc fx.Lifecycle) exchange.Interface {
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
