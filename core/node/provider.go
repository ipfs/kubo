package node

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-routing"
	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/core/node/helpers"
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
func ReproviderCtor(reproviderInterval time.Duration) func(helpers.MetricsCtx, fx.Lifecycle, routing.IpfsRouting, reprovide.KeyChanFunc) (*reprovide.Reprovider, error) {
	return func(mctx helpers.MetricsCtx, lc fx.Lifecycle, rt routing.IpfsRouting, keyProvider reprovide.KeyChanFunc) (*reprovide.Reprovider, error) {
		return reprovide.NewReprovider(helpers.LifecycleCtx(mctx, lc), reproviderInterval, rt, keyProvider), nil
	}
}

// Reprovider runs the reprovider service
func Reprovider(lp lcProcess, reprovider *reprovide.Reprovider) error {
	lp.Append(reprovider.Run)
	return nil
}
