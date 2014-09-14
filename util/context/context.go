package context

import (
	"time"

	goctx "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

// Context adds additional functionality to the go.net context
type Context interface {
	goctx.Context
	// LogError sends error information to the actor who instantiated the
	// context. (non-blocking)
	LogError(error)
}

func WithCancel(
	parent goctx.Context) (Context, goctx.CancelFunc) {

	ctx, cancelFunc := goctx.WithCancel(parent)
	w := wrap(ctx)
	return w, cancelFunc
}

func WithDeadline(
	parent goctx.Context, deadline time.Time) (Context, goctx.CancelFunc) {

	ctx, cancelFunc := goctx.WithDeadline(parent, deadline)
	w := wrap(ctx)
	return w, cancelFunc
}

func WithTimeout(
	parent goctx.Context, timeout time.Duration) (Context, goctx.CancelFunc) {

	ctx, cancelFunc := goctx.WithTimeout(parent, timeout)
	w := wrap(ctx)
	return w, cancelFunc
}

func WithValue(
	parent goctx.Context, key interface{}, val interface{}) Context {

	ctx := goctx.WithValue(parent, key, val)
	w := wrap(ctx)
	return w
}

func wrap(ctx goctx.Context) *wrappedContext {
	w := &wrappedContext{Context: ctx}
	return w
}

// wrappedContext implements this package's Context interface
type wrappedContext struct {
	goctx.Context
}

// NB: this function must _never_ block the caller
func (c *wrappedContext) LogError(err error) {
	// TODO(brian): implement
}
