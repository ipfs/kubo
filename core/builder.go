package core

import (
	"context"

	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/core/bootstrap"
	"github.com/ipfs/go-ipfs/core/node"
)

type BuildCfg = node.BuildCfg // Alias for compatibility until we properly refactor the constructor interface

// NewNode constructs and returns an IpfsNode using the given cfg.
func NewNode(ctx context.Context, cfg *BuildCfg) (*IpfsNode, error) {
	n := &IpfsNode{
		ctx: ctx,
	}

	app := fx.New(
		node.IPFS(ctx, cfg),

		fx.NopLogger,
		fx.Extract(n),
	)

	go func() {
		// Note that some services use contexts to signal shutting down, which is
		// very suboptimal. This needs to be here until that's addressed somehow
		<-ctx.Done()
		app.Stop(context.Background())
	}()

	n.IsOnline = cfg.Online
	n.app = app

	if app.Err() != nil {
		return nil, app.Err()
	}

	if err := app.Start(ctx); err != nil {
		return nil, err
	}

	// TODO: How soon will bootstrap move to libp2p?
	if !cfg.Online {
		return n, nil
	}

	return n, n.Bootstrap(bootstrap.DefaultBootstrapConfig)
}
