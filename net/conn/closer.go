package conn

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

// Wait is a readable channel to block on until it receives a signal.
type Wait <-chan Signal

// Signal is an empty channel
type Signal struct{}

// CloseFunc is a function used to close a ContextCloser
type CloseFunc func() error

// ContextCloser is an interface for services able to be opened and closed.
type ContextCloser interface {
	Context() context.Context

	// Close is a method to call when you with to stop this ContextCloser
	Close() error

	// Done is a method to wait upon, like context.Context.Done
	Done() Wait
}

// contextCloser is an OpenCloser with a cancellable context
type contextCloser struct {
	ctx    context.Context
	cancel context.CancelFunc

	// called to close
	closeFunc CloseFunc

	// closed is released once the close function is done.
	closed chan Signal
}

// NewContextCloser constructs and returns a ContextCloser. It will call
// cf CloseFunc before its Done() Wait signals fire.
func NewContextCloser(ctx context.Context, cf CloseFunc) ContextCloser {
	ctx, cancel := context.WithCancel(ctx)
	c := &contextCloser{
		ctx:       ctx,
		cancel:    cancel,
		closeFunc: cf,
		closed:    make(chan Signal),
	}

	go c.closeOnContextDone()
	return c
}

func (c *contextCloser) Context() context.Context {
	return c.ctx
}

func (c *contextCloser) Done() Wait {
	return c.closed
}

func (c *contextCloser) Close() error {
	select {
	case <-c.Done():
		panic("closed twice")
	default:
	}

	c.cancel()           // release anyone waiting on the context
	err := c.closeFunc() // actually run the close logic
	close(c.closed)      // relase everyone waiting on Done
	return err
}

func (c *contextCloser) closeOnContextDone() {
	<-c.ctx.Done()
	select {
	case <-c.Done():
		return // already closed
	default:
	}
	c.Close()
}
