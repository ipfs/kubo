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

// WithErrorLog ignores the parent's logging behavior. Returns a channel from the
func WithErrorLog(parent goctx.Context) (Context, <-chan error) {
	wrapper, _ := wrap(parent, ignoredFunc)
	wrapper.LoggingErrors(true)
	return wrapper, wrapper.Errs()
}

func WithCancel(parent goctx.Context) (Context, CancelFunc) {
	generator := func() (goctx.Context, goctx.CancelFunc) {
		return goctx.WithCancel(parent)
	}
	return z(parent, generator)
}

func WithDeadline(
	parent goctx.Context, deadline time.Time) (Context, CancelFunc) {

	generator := func() (goctx.Context, goctx.CancelFunc) {
		return goctx.WithDeadline(parent, deadline)
	}
	return z(parent, generator)
}

func WithTimeout(
	parent goctx.Context, timeout time.Duration) (Context, CancelFunc) {

	generator := func() (goctx.Context, goctx.CancelFunc) {
		return goctx.WithTimeout(parent, timeout)
	}
	return z(parent, generator)
}

func WithValue(
	parent goctx.Context, key interface{}, val interface{}) Context {

	generator := func() (goctx.Context, goctx.CancelFunc) {
		return goctx.WithValue(parent, key, val), ignoredFunc
	}
	ctx, _ := z(parent, generator)
	return ctx
}

type ctxGeneratorFunc func() (goctx.Context, goctx.CancelFunc)

// z ensures behavior is appropriately passed from parent |wc| to child iff
// parent is a wrapped context and parent is logging
// |g| is a context generator function used to generate the child
func z(parent goctx.Context, g ctxGeneratorFunc) (Context, CancelFunc) {
	wc, isAWrappedContext := parent.(*wrappedContext)
	if !isAWrappedContext {
		// can happen when getting a context from another library
		return wrap(g())
	}
	if !wc.loggingErrors {
		return wrap(g())
	}
	child, cancelFunc := wrap(g())
	child.LoggingErrors(true)
	child.ErrLogChan(wc.Errs())
	return child, cancelFunc
}

// wrap() wraps |ctx| to extend its interface and turns |f| into a CancelFunc
func wrap(ctx goctx.Context, f goctx.CancelFunc) (*wrappedContext, CancelFunc) {
	w := &wrappedContext{Context: ctx, errLogChan: make(chan error)}
	return w, CancelFunc(f)
}

// ignoredFunc is a placeholder value to be used in calls to wrap (when the
// goctx factory method doesn't include a cancelFunc)
var ignoredFunc = func() {}

// wrappedContext implements this package's Context interface
type wrappedContext struct {
	goctx.Context
	errLogChan    chan error
	loggingErrors bool
}

func (ctx *wrappedContext) LogError(err error) {
	if ctx.loggingErrors {
		ctx.errLogChan <- err
	}
}

func (ctx *wrappedContext) LoggingErrors(b bool) {
	ctx.loggingErrors = b
}

func (ctx *wrappedContext) Errs() chan error {
	return ctx.errLogChan
}

func (ctx *wrappedContext) ErrLogChan(ch chan error) {
	ctx.errLogChan = ch
}
