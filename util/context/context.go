package context

import (
	"time"

	goctx "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

// TODO(brian): add logging
type Context interface {
	goctx.Context

	// LogError sends error information to the actor who instantiated the
	// context. (non-blocking)
	LogError(error)
}

// A CancelFunc tells an operation to abandon its work.
type CancelFunc goctx.CancelFunc

// Background returns a non-nil, empty Context. It is never canceled, has no
// values, and has no deadline.
func Background() Context {
	ctx, _ := wrap(goctx.Background(), ignoredFunc)
	return ctx
}

// WithErrorLog derives a new logging context. The returned error channel
// receives errors from the returned context as well as any other descendant
// contexts derived from the returned context. However, if a descendant context
// |d| is derived using WithErrorLog, then the error channel associated with
// |d| will capture errors for |d| and contexts derived from |d|.
func WithErrorLog(parent goctx.Context) (Context, <-chan error) {
	wrapper, _ := wrap(parent, ignoredFunc)
	wrapper.LoggingErrors(true)
	return wrapper, wrapper.Errs()
}

// WithCancel derives a cancellable context
func WithCancel(parent goctx.Context) (Context, CancelFunc) {
	generator := func() (goctx.Context, goctx.CancelFunc) {
		return goctx.WithCancel(parent)
	}
	return deriveFrom(parent, generator)
}

func WithDeadline(
	parent goctx.Context, deadline time.Time) (Context, CancelFunc) {

	generator := func() (goctx.Context, goctx.CancelFunc) {
		return goctx.WithDeadline(parent, deadline)
	}
	return deriveFrom(parent, generator)
}

func WithTimeout(
	parent goctx.Context, timeout time.Duration) (Context, CancelFunc) {

	generator := func() (goctx.Context, goctx.CancelFunc) {
		return goctx.WithTimeout(parent, timeout)
	}
	return deriveFrom(parent, generator)
}

func WithValue(
	parent goctx.Context, key interface{}, val interface{}) Context {

	generator := func() (goctx.Context, goctx.CancelFunc) {
		return goctx.WithValue(parent, key, val), ignoredFunc
	}
	ctx, _ := deriveFrom(parent, generator)
	return ctx
}

// ctxGeneratorFuncs are used to make it possible to share the deriveFrom
// function despite the fact that each public factory method passes different
// parameters. Thus ctxGeneratorFuncs are best implemented as closures.
type ctxGeneratorFunc func() (goctx.Context, goctx.CancelFunc)

// deriveFrom() derives a new child context from |parent| using the generator
// function |g|.
//
// Furthermore, deriveFrom() ensures behavior is appropriately passed from a
// parent wrapped context to a child wrapped context.
//
// If |wc| has logging enabled, the child derived from |wc| will inherit wc's
// behavior.
func deriveFrom(parent goctx.Context, g ctxGeneratorFunc) (Context, CancelFunc) {
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
