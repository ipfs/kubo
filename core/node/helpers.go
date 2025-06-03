package node

import (
	"context"
	"errors"

	"github.com/jbenet/goprocess"
	"go.uber.org/fx"
)

type lcStartStop struct {
	fx.In

	LC fx.Lifecycle
}

// Append wraps a function into a fx.Hook and appends it to the fx.Lifecycle.
func (lcss *lcStartStop) Append(f func() func()) {
	// Hooks are guaranteed to run in sequence. If a hook fails to start, its
	// OnStop won't be executed.
	var stopFunc func()

	lcss.LC.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if ctx.Err() != nil {
				return nil
			}
			stopFunc = f()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if ctx.Err() != nil {
				return nil
			}
			if stopFunc == nil { // Theoretically this shouldn't ever happen
				return errors.New("lcStatStop: stopFunc was nil")
			}
			stopFunc()
			return nil
		},
	})
}

func maybeProvide(opt interface{}, enable bool) fx.Option {
	if enable {
		return fx.Provide(opt)
	}
	return fx.Options()
}

// nolint unused
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
