package core

import (
	"context"
	"sync"
	"time"

	"github.com/ipfs/go-ipfs/core/bootstrap"
	"github.com/ipfs/go-ipfs/core/node"

	"github.com/ipfs/go-metrics-interface"
	"go.uber.org/fx"
)

// from https://stackoverflow.com/a/59348871
type valueContext struct {
	context.Context
}

func (valueContext) Deadline() (deadline time.Time, ok bool) { return }
func (valueContext) Done() <-chan struct{}                   { return nil }
func (valueContext) Err() error                              { return nil }

type BuildCfg = node.BuildCfg // Alias for compatibility until we properly refactor the constructor interface

// NewNode constructs and returns an IpfsNode using the given cfg.
func NewNode(ctx context.Context, cfg *BuildCfg) (*IpfsNode, error) {
	// save this context as the "lifetime" ctx.
	lctx := ctx

	// derive a new context that ignores cancellations from the lifetime ctx.
	ctx, cancel := context.WithCancel(valueContext{ctx})

	// add a metrics scope.
	ctx = metrics.CtxScope(ctx, "ipfs")

	n := &IpfsNode{
		ctx: ctx,
	}

	app := fx.New(
		node.IPFS(ctx, cfg),

		fx.NopLogger,
		fx.Extract(n),
	)

	var once sync.Once
	var stopErr error
	n.stop = func() error {
		once.Do(func() {
			stopErr = app.Stop(context.Background())
			if stopErr != nil {
				log.Error("failure on stop: ", stopErr)
			}
			// Cancel the context _after_ the app has stopped.
			cancel()
		})
		return stopErr
	}
	n.IsOnline = cfg.Online

	go func() {
		// Shut down the application if the lifetime context is canceled.
		// NOTE: we _should_ stop the application by calling `Close()`
		// on the process. But we currently manage everything with contexts.
		select {
		case <-lctx.Done():
			err := n.stop()
			if err != nil {
				log.Error("failure on stop: ", err)
			}
		case <-ctx.Done():
		}
	}()

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
