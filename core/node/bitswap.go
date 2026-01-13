package node

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/ipfs/boxo/bitswap"
	"github.com/ipfs/boxo/bitswap/client"
	"github.com/ipfs/boxo/bitswap/network"
	bsnet "github.com/ipfs/boxo/bitswap/network/bsnet"
	"github.com/ipfs/boxo/bitswap/network/httpnet"
	blockstore "github.com/ipfs/boxo/blockstore"
	exchange "github.com/ipfs/boxo/exchange"
	rpqm "github.com/ipfs/boxo/routing/providerquerymanager"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	version "github.com/ipfs/kubo"
	"github.com/ipfs/kubo/config"
	"github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
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
	Discovery   routing.ContentDiscovery
	Bs          blockstore.GCBlockstore
	BitswapOpts []bitswap.Option `group:"bitswap-options"`
}

// Bitswap creates the BitSwap server/client instance.
// If Bitswap.ServerEnabled is false, the node will act only as a client
// using an empty blockstore to prevent serving blocks to other peers.
func Bitswap(serverEnabled, libp2pEnabled, httpEnabled bool) interface{} {
	return func(in bitswapIn, lc fx.Lifecycle) (*bitswap.Bitswap, error) {
		var bitswapNetworks, bitswapLibp2p network.BitSwapNetwork
		var bitswapBlockstore blockstore.Blockstore = in.Bs

		connEvtMgr := network.NewConnectEventManager()

		libp2pEnabled := in.Cfg.Bitswap.Libp2pEnabled.WithDefault(config.DefaultBitswapLibp2pEnabled)
		if libp2pEnabled {
			bitswapLibp2p = bsnet.NewFromIpfsHost(
				in.Host,
				bsnet.WithConnectEventManager(connEvtMgr),
			)
		}

		if httpEnabled {
			httpCfg := in.Cfg.HTTPRetrieval
			maxBlockSize, err := humanize.ParseBytes(httpCfg.MaxBlockSize.WithDefault(config.DefaultHTTPRetrievalMaxBlockSize))
			if err != nil {
				return nil, err
			}
			logger.Infof("HTTP Retrieval enabled: Allowlist: %t. Denylist: %t",
				httpCfg.Allowlist != nil,
				httpCfg.Denylist != nil,
			)

			bitswapHTTP := httpnet.New(in.Host,
				httpnet.WithHTTPWorkers(int(httpCfg.NumWorkers.WithDefault(config.DefaultHTTPRetrievalNumWorkers))),
				httpnet.WithAllowlist(httpCfg.Allowlist),
				httpnet.WithDenylist(httpCfg.Denylist),
				httpnet.WithInsecureSkipVerify(httpCfg.TLSInsecureSkipVerify.WithDefault(config.DefaultHTTPRetrievalTLSInsecureSkipVerify)),
				httpnet.WithMaxBlockSize(int64(maxBlockSize)),
				httpnet.WithUserAgent(version.GetUserAgentVersion()),
				httpnet.WithMetricsLabelsForEndpoints(httpCfg.Allowlist),
				httpnet.WithConnectEventManager(connEvtMgr),
			)
			bitswapNetworks = network.New(in.Host.Peerstore(), bitswapLibp2p, bitswapHTTP)
		} else if libp2pEnabled {
			bitswapNetworks = bitswapLibp2p
		} else {
			return nil, errors.New("invalid configuration: Bitswap.Libp2pEnabled and HTTPRetrieval.Enabled are both disabled, unable to initialize Bitswap")
		}

		// Kubo uses own, customized ProviderQueryManager
		in.BitswapOpts = append(in.BitswapOpts, bitswap.WithClientOption(client.WithDefaultProviderQueryManager(false)))
		var maxProviders int = DefaultMaxProviders

		var bcDisposition string
		if in.Cfg.Internal.Bitswap != nil {
			maxProviders = int(in.Cfg.Internal.Bitswap.ProviderSearchMaxResults.WithDefault(DefaultMaxProviders))
			if in.Cfg.Internal.Bitswap.BroadcastControl != nil {
				bcCfg := in.Cfg.Internal.Bitswap.BroadcastControl
				bcEnable := bcCfg.Enable.WithDefault(config.DefaultBroadcastControlEnable)
				in.BitswapOpts = append(in.BitswapOpts, bitswap.WithClientOption(client.BroadcastControlEnable(bcEnable)))
				if bcEnable {
					bcDisposition = "enabled"
					bcMaxPeers := int(bcCfg.MaxPeers.WithDefault(config.DefaultBroadcastControlMaxPeers))
					in.BitswapOpts = append(in.BitswapOpts, bitswap.WithClientOption(client.BroadcastControlMaxPeers(bcMaxPeers)))

					bcLocalPeers := bcCfg.LocalPeers.WithDefault(config.DefaultBroadcastControlLocalPeers)
					in.BitswapOpts = append(in.BitswapOpts, bitswap.WithClientOption(client.BroadcastControlLocalPeers(bcLocalPeers)))

					bcPeeredPeers := bcCfg.PeeredPeers.WithDefault(config.DefaultBroadcastControlPeeredPeers)
					in.BitswapOpts = append(in.BitswapOpts, bitswap.WithClientOption(client.BroadcastControlPeeredPeers(bcPeeredPeers)))

					bcMaxRandomPeers := int(bcCfg.MaxRandomPeers.WithDefault(config.DefaultBroadcastControlMaxRandomPeers))
					in.BitswapOpts = append(in.BitswapOpts, bitswap.WithClientOption(client.BroadcastControlMaxRandomPeers(bcMaxRandomPeers)))

					bcSendToPendingPeers := bcCfg.SendToPendingPeers.WithDefault(config.DefaultBroadcastControlSendToPendingPeers)
					in.BitswapOpts = append(in.BitswapOpts, bitswap.WithClientOption(client.BroadcastControlSendToPendingPeers(bcSendToPendingPeers)))
				} else {
					bcDisposition = "disabled"
				}
			}
		}

		// If broadcast control is not configured, then configure with defaults.
		if bcDisposition == "" {
			in.BitswapOpts = append(in.BitswapOpts, bitswap.WithClientOption(client.BroadcastControlEnable(config.DefaultBroadcastControlEnable)))
			if config.DefaultBroadcastControlEnable {
				bcDisposition = "enabled"
				in.BitswapOpts = append(in.BitswapOpts, bitswap.WithClientOption(client.BroadcastControlMaxPeers(config.DefaultBroadcastControlMaxPeers)))
				in.BitswapOpts = append(in.BitswapOpts, bitswap.WithClientOption(client.BroadcastControlLocalPeers(config.DefaultBroadcastControlLocalPeers)))
				in.BitswapOpts = append(in.BitswapOpts, bitswap.WithClientOption(client.BroadcastControlPeeredPeers(config.DefaultBroadcastControlPeeredPeers)))
				in.BitswapOpts = append(in.BitswapOpts, bitswap.WithClientOption(client.BroadcastControlMaxRandomPeers(config.DefaultBroadcastControlMaxRandomPeers)))
				in.BitswapOpts = append(in.BitswapOpts, bitswap.WithClientOption(client.BroadcastControlSendToPendingPeers(config.DefaultBroadcastControlSendToPendingPeers)))
			} else {
				bcDisposition = "enabled"
			}
		}
		logger.Infof("bitswap client broadcast control %s", bcDisposition)

		ignoredPeerIDs := make([]peer.ID, 0, len(in.Cfg.Routing.IgnoreProviders))
		for _, str := range in.Cfg.Routing.IgnoreProviders {
			pid, err := peer.Decode(str)
			if err != nil {
				return nil, err
			}
			ignoredPeerIDs = append(ignoredPeerIDs, pid)
		}
		providerQueryMgr, err := rpqm.New(bitswapNetworks,
			in.Discovery,
			rpqm.WithMaxProviders(maxProviders),
			rpqm.WithIgnoreProviders(ignoredPeerIDs...),
		)
		if err != nil {
			return nil, err
		}

		// Explicitly enable/disable server
		in.BitswapOpts = append(in.BitswapOpts, bitswap.WithServerEnabled(serverEnabled))

		bs := bitswap.New(helpers.LifecycleCtx(in.Mctx, lc), bitswapNetworks, providerQueryMgr, bitswapBlockstore, in.BitswapOpts...)

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

type noopExchange struct {
	closer io.Closer
}

func (e *noopExchange) GetBlock(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	return nil, ipld.ErrNotFound{Cid: c}
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
