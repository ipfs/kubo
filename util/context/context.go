package context

import (
	"time"

	goctx "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

// Context is a drop-in extension to the go.net context. It adds error
// reporting and general logging functionality.
// TODO(brian): add logging
type Context interface {
	goctx.Context
	// LogError sends error information to the actor who instantiated the
	// context. (non-blocking)
	LogError(error)
}

type CancelFunc goctx.CancelFunc

func Background() Context {
	ctx, _ := wrap(goctx.Background(), ignoredFunc)
	return ctx
}

func WithCancel(
	parent goctx.Context) (Context, CancelFunc) {

	return wrap(goctx.WithCancel(parent))
}

func WithDeadline(
	parent goctx.Context, deadline time.Time) (Context, CancelFunc) {

	return wrap(goctx.WithDeadline(parent, deadline))
}

func WithTimeout(
	parent goctx.Context, timeout time.Duration) (Context, CancelFunc) {

	return wrap(goctx.WithTimeout(parent, timeout))
}

func WithValue(
	parent goctx.Context, key interface{}, val interface{}) Context {

	ctx, _ := wrap(goctx.WithValue(parent, key, val), ignoredFunc)
	return ctx
}

// wrap() wraps |ctx| to extend its interface and turns |f| into a CancelFunc
func wrap(ctx goctx.Context, f goctx.CancelFunc) (*wrappedContext, CancelFunc) {
	w := &wrappedContext{Context: ctx}
	return w, CancelFunc(f)
}

// ignoredFunc is a placeholder value to be used in calls to wrap (when the
// goctx factory method doesn't include a cancelFunc)
var ignoredFunc = func() {}

// wrappedContext implements this package's Context interface
type wrappedContext struct {
	goctx.Context
}

// NB: this function must _never_ block the caller
func (c *wrappedContext) LogError(err error) {
	// TODO(brian): implement
}
