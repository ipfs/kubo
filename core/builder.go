package core

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/ipfs/boxo/bootstrap"
	"github.com/ipfs/kubo/core/node"

	"github.com/ipfs/go-metrics-interface"
	"go.uber.org/dig"
	"go.uber.org/fx"
)

// FXNodeInfo contains information useful for adding fx options.
// This is the extension point for providing more info/context to fx plugins
// to make decisions about what options to include.
type FXNodeInfo struct {
	FXOptions []fx.Option
}

// fxOptFunc takes in some info about the IPFS node and returns the full set of fx opts to use.
type fxOptFunc func(FXNodeInfo) ([]fx.Option, error)

var fxOptionFuncs []fxOptFunc

// RegisterFXOptionFunc registers a function that is run before the fx app is initialized.
// Functions are invoked in the order they are registered,
// and the resulting options are passed into the next function's FXNodeInfo.
//
// Note that these are applied globally, by all invocations of NewNode.
// There are multiple places in Kubo that construct nodes, such as:
//   - Repo initialization
//   - Daemon initialization
//   - When running migrations
//   - etc.
//
// If your fx options are doing anything sophisticated, you should keep this in mind.
//
// For example, if you plug in a blockservice that disallows non-allowlisted CIDs,
// this may break migrations that fetch migration code over IPFS.
func RegisterFXOptionFunc(optFunc fxOptFunc) {
	fxOptionFuncs = append(fxOptionFuncs, optFunc)
}

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

	opts := []fx.Option{
		node.IPFS(ctx, cfg),
		fx.NopLogger,
	}
	for _, optFunc := range fxOptionFuncs {
		var err error
		opts, err = optFunc(FXNodeInfo{FXOptions: opts})
		if err != nil {
			cancel()
			return nil, fmt.Errorf("building fx opts: %w", err)
		}
	}
	//nolint:staticcheck // https://github.com/ipfs/kubo/pull/9423#issuecomment-1341038770
	opts = append(opts, fx.Extract(n))

	app := fx.New(opts...)

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
		return nil, logAndUnwrapFxError(app.Err())
	}

	if err := app.Start(ctx); err != nil {
		return nil, logAndUnwrapFxError(err)
	}

	// TODO: How soon will bootstrap move to libp2p?
	if !cfg.Online {
		return n, nil
	}

	return n, n.Bootstrap(bootstrap.DefaultBootstrapConfig)
}

// Log the entire `app.Err()` but return only the innermost one to the user
// given the full error can be very long (as it can expose the entire build
// graph in a single string).
//
// The fx.App error exposed through `app.Err()` normally contains un-exported
// errors from its low-level `dig` package:
// * https://github.com/uber-go/dig/blob/5e5a20d/error.go#L82
// These usually wrap themselves in many layers to expose where in the build
// chain did the error happen. Although useful for a developer that needs to
// debug it, it can be very confusing for a user that just wants the IPFS error
// that he can probably fix without being aware of the entire chain.
// Unwrapping everything is not the best solution as there can be useful
// information in the intermediate errors, mainly in the next to last error
// that locates which component is the build error coming from, but it's the
// best we can do at the moment given all errors in dig are private and we
// just have the generic `RootCause` API.
func logAndUnwrapFxError(fxAppErr error) error {
	if fxAppErr == nil {
		return nil
	}

	log.Error("constructing the node: ", fxAppErr)

	err := fxAppErr
	for {
		extractedErr := dig.RootCause(err)
		// Note that the `RootCause` name is misleading as it just unwraps only
		// *one* error layer at a time, so we need to continuously call it.
		if !reflect.TypeOf(extractedErr).Comparable() {
			// Some internal errors are not comparable (e.g., `dig.errMissingTypes`
			// which is a slice) and we can't go further.
			break
		}
		if extractedErr == err {
			// We didn't unwrap any new error in the last call, reached the innermost one.
			break
		}
		err = extractedErr
	}

	return fmt.Errorf("constructing the node (see log for full detail): %w", err)
}
