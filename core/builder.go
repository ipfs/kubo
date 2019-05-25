package core

import (
	"context"

	"github.com/ipfs/go-metrics-interface"
	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/core/bootstrap"
	"github.com/ipfs/go-ipfs/core/node"
	"github.com/jbenet/goprocess/context"
)

type BuildCfg = node.BuildCfg // Alias for compatibility until we properly refactor the constructor interface

// NewNode constructs and returns an IpfsNode using the given cfg.
func NewNode(ctx context.Context, cfg *BuildCfg) (*IpfsNode, error) {
	ctx = metrics.CtxScope(ctx, "ipfs")

	n := &IpfsNode{
		ctx: ctx,
	}

	app := fx.New(
		node.IPFS(ctx, cfg),

		fx.NopLogger,
		fx.Extract(n),
	)

	n.IsOnline = cfg.Online

	if app.Err() != nil {
		return nil, app.Err()
	}

	// bind the process to the context so we stop when the context is
	// canceled
	goprocessctx.CloseAfterContext(n.Process, ctx)

	if err := app.Start(ctx); err != nil {
		stopErr := app.Stop(context.Background())
		log.Error("failure on stop: ", stopErr)
		return nil, err
	}

	// TODO: How soon will bootstrap move to libp2p?
	if !cfg.Online {
		return n, nil
	}

	return n, n.Bootstrap(bootstrap.DefaultBootstrapConfig)
}
