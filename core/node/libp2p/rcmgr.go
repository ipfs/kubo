package libp2p

import (
	"context"
	"fmt"
	"github.com/ipfs/go-ipfs/repo"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/network"
	rcmgr "github.com/libp2p/go-libp2p-resource-manager"
	"go.uber.org/fx"
)

func ResourceManager() func(fx.Lifecycle, repo.Repo) (network.ResourceManager, Libp2pOpts, error) {
	return func(lc fx.Lifecycle, repo repo.Repo) (network.ResourceManager, Libp2pOpts, error) {
		var limiter *rcmgr.BasicLimiter
		var opts Libp2pOpts

		// FIXME(BLOCKING): Decide how is the `limit.json` file path going to be consumed,
		//  either by default in the repo root or through the `go-ipfs-config`.
		limiter = rcmgr.NewDefaultLimiter()

		libp2p.SetDefaultServiceLimits(limiter)

		rcmgr, err := rcmgr.NewResourceManager(limiter)
		if err != nil {
			return nil, opts, fmt.Errorf("error creating resource manager: %w", err)
		}
		opts.Opts = append(opts.Opts, libp2p.ResourceManager(rcmgr))

		lc.Append(fx.Hook{
			OnStop: func(_ context.Context) error {
				return rcmgr.Close()
			}})

		return rcmgr, opts, nil
	}
}
