package ctxutil

import (
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

func WithDeadlineFraction(ctx context.Context, fraction float64) (context.Context, context.CancelFunc) {
	d, found := ctx.Deadline()
	if !found { // no deadline
		return context.WithCancel(ctx)
	}

	left := d.Sub(time.Now())
	if left < 0 { // already passed...
		return context.WithCancel(ctx)
	}

	left = time.Duration(float64(left) * fraction)
	return context.WithTimeout(ctx, left)
}
