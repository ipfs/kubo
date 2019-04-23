package node

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-ipld-format"
	"github.com/libp2p/go-libp2p-routing"
	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/core/node/helpers"
	"github.com/ipfs/go-ipfs/pin"
	"github.com/ipfs/go-ipfs/provider"
	"github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/reprovide"
)

const kReprovideFrequency = time.Hour * 12

// ProviderQueue creates new datastore backed provider queue
func ProviderQueue(mctx helpers.MetricsCtx, lc fx.Lifecycle, repo repo.Repo) (*provider.Queue, error) {
	return provider.NewQueue(helpers.LifecycleCtx(mctx, lc), "provider-v1", repo.Datastore())
}

// ProviderCtor creates new record provider
func ProviderCtor(mctx helpers.MetricsCtx, lc fx.Lifecycle, queue *provider.Queue, rt routing.IpfsRouting) provider.Provider {
	p := provider.NewProvider(helpers.LifecycleCtx(mctx, lc), queue, rt)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			p.Run()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return p.Close()
		},
	})

	return p
}

// ReproviderCtor creates new reprovider
func ReproviderCtor(mctx helpers.MetricsCtx, lc fx.Lifecycle, cfg *config.Config, bs BaseBlocks, ds format.DAGService, pinning pin.Pinner, rt routing.IpfsRouting) (*reprovide.Reprovider, error) {
	var keyProvider reprovide.KeyChanFunc

	reproviderInterval := kReprovideFrequency
	if cfg.Reprovider.Interval != "" {
		dur, err := time.ParseDuration(cfg.Reprovider.Interval)
		if err != nil {
			return nil, err
		}

		reproviderInterval = dur
	}

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
	return reprovide.NewReprovider(helpers.LifecycleCtx(mctx, lc), reproviderInterval, rt, keyProvider), nil
}

// Reprovider runs the reprovider service
func Reprovider(lp lcProcess, reprovider *reprovide.Reprovider) error {
	lp.Append(reprovider.Run)
	return nil
}
