package node

import (
	"context"

	"github.com/jbenet/goprocess"
	"github.com/pkg/errors"
	"go.uber.org/fx"
)

type lcProcess struct {
	fx.In

	LC   fx.Lifecycle
	Proc goprocess.Process
}

// Append wraps ProcessFunc into a goprocess, and appends it to the lifecycle
func (lp *lcProcess) Append(f goprocess.ProcessFunc) {
	// Hooks are guaranteed to run in sequence. If a hook fails to start, its
	// OnStop won't be executed.
	var proc goprocess.Process

	lp.LC.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			proc = lp.Proc.Go(f)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if proc == nil { // Theoretically this shouldn't ever happen
				return errors.New("lcProcess: proc was nil")
			}

			return proc.Close() // todo: respect ctx, somehow
		},
	})
}

func maybeProvide(opt interface{}, enable bool) fx.Option {
	if enable {
		return fx.Provide(opt)
	}
	return fx.Options()
}

//nolint unused
func maybeInvoke(opt interface{}, enable bool) fx.Option {
	if enable {
		return fx.Invoke(opt)
	}
	return fx.Options()
}

// baseProcess creates a goprocess which is closed when the lifecycle signals it to stop
func baseProcess(lc fx.Lifecycle) goprocess.Process {
	p := goprocess.WithParent(goprocess.Background())
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			return p.Close()
		},
	})
	return p
}
