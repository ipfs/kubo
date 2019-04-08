package node

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-ipld-format"
	"github.com/libp2p/go-libp2p-routing"
	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/exchange/reprovide"
	"github.com/ipfs/go-ipfs/pin"
	"github.com/ipfs/go-ipfs/provider"
	"github.com/ipfs/go-ipfs/repo"
)

const kReprovideFrequency = time.Hour * 12

func ProviderQueue(mctx MetricsCtx, lc fx.Lifecycle, repo repo.Repo) (*provider.Queue, error) {
	return provider.NewQueue(lifecycleCtx(mctx, lc), "provider-v1", repo.Datastore())
}

func ProviderCtor(mctx MetricsCtx, lc fx.Lifecycle, queue *provider.Queue, rt routing.IpfsRouting) provider.Provider {
	p := provider.NewProvider(lifecycleCtx(mctx, lc), queue, rt)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			p.Run()
			return nil
		},
	})

	return p
}

func ReproviderCtor(mctx MetricsCtx, lc fx.Lifecycle, cfg *config.Config, bs BaseBlocks, ds format.DAGService, pinning pin.Pinner, rt routing.IpfsRouting) (*reprovide.Reprovider, error) {
	var keyProvider reprovide.KeyChanFunc

	switch cfg.Reprovider.Strategy {
	case "all":
		fallthrough
	case "":
		keyProvider = reprovide.NewBlockstoreProvider(bs)
	case "roots":
		keyProvider = reprovide.NewPinnedProvider(pinning, ds, true)
	case "pinned":
		keyProvider = reprovide.NewPinnedProvider(pinning, ds, false)
	default:
		return nil, fmt.Errorf("unknown reprovider strategy '%s'", cfg.Reprovider.Strategy)
	}
	return reprovide.NewReprovider(lifecycleCtx(mctx, lc), rt, keyProvider), nil
}

func Reprovider(cfg *config.Config, reprovider *reprovide.Reprovider) error {
	reproviderInterval := kReprovideFrequency
	if cfg.Reprovider.Interval != "" {
		dur, err := time.ParseDuration(cfg.Reprovider.Interval)
		if err != nil {
			return err
		}

		reproviderInterval = dur
	}

	go reprovider.Run(reproviderInterval)
	return nil
}
