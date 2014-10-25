package ctxcloser

import (
	"sync"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

// CloseFunc is a function used to close a ContextCloser
type CloseFunc func() error

var nilCloseFunc = func() error { return nil }

// ContextCloser is an interface for services able to be opened and closed.
// It has a parent Context, and Children. But ContextCloser is not a proper
// "tree" like the Context tree. It is more like a Context-WaitGroup hybrid.
// It models a main object with a few children objects -- and, unlike the
// context -- concerns itself with the parent-child closing semantics:
//
// - Can define a CloseFunc (func() error) to be run at Close time.
// - Children call Children().Add(1) to be waited upon
// - Children can select on <-Closing() to know when they should shut down.
// - Close() will wait until all children call Children().Done()
// - <-Closed() signals when the service is completely closed.
//
// ContextCloser can be embedded into the main object itself. In that case,
// the closeFunc (if a member function) has to be set after the struct
// is intialized:
//
//  type service struct {
//  	ContextCloser
//  	net.Conn
//  }
//
//  func (s *service) close() error {
//  	return s.Conn.Close()
//  }
//
//  func newService(ctx context.Context, c net.Conn) *service {
//  	s := &service{c}
//  	s.ContextCloser = NewContextCloser(ctx, s.close)
//  	return s
//  }
//
type ContextCloser interface {

	// Context is the context of this ContextCloser. It is "sort of" a parent.
	Context() context.Context

	// Children is a sync.Waitgroup for all children goroutines that should
	// shut down completely before this service is said to be "closed".
	// Follows the semantics of WaitGroup:
	//
	//  Children().Add(1) // add one more dependent child
	//  Children().Done() // child signals it is done
	//
	Children() *sync.WaitGroup

	// AddCloserChild registers a dependent ContextCloser child. The child will
	// be closed when this parent is closed, and waited upon to finish. It is
	// the functional equivalent of the following:
	//
	//  go func(parent, child ContextCloser) {
	//  	parent.Children().Add(1) // add one more dependent child
	//  	<-parent.Closing()       // wait until parent is closing
	//  	child.Close()            // signal child to close
	//  	parent.Children().Done() // child signals it is done
	//	}(a, b)
	//
	AddCloserChild(c ContextCloser)

	// Close is a method to call when you wish to stop this ContextCloser
	Close() error

	// Closing is a signal to wait upon, like Context.Done().
	// It fires when the object should be closing (but hasn't yet fully closed).
	// The primary use case is for child goroutines who need to know when
	// they should shut down. (equivalent to Context().Done())
	Closing() <-chan struct{}

	// Closed is a method to wait upon, like Context.Done().
	// It fires when the entire object is fully closed.
	// The primary use case is for external listeners who need to know when
	// this object is completly done, and all its children closed.
	Closed() <-chan struct{}
}

// contextCloser is an OpenCloser with a cancellable context
type contextCloser struct {
	ctx    context.Context
	cancel context.CancelFunc

	// called to run the close logic.
	closeFunc CloseFunc

	// closed is released once the close function is done.
	closed chan struct{}

	// wait group for child goroutines
	children sync.WaitGroup

	// sync primitive to ensure the close logic is only called once.
	closeOnce sync.Once

	// error to return to clients of Close().
	closeErr error
}

// NewContextCloser constructs and returns a ContextCloser. It will call
// cf CloseFunc before its Done() Wait signals fire.
func NewContextCloser(ctx context.Context, cf CloseFunc) ContextCloser {
	if cf == nil {
		cf = nilCloseFunc
	}
	ctx, cancel := context.WithCancel(ctx)
	c := &contextCloser{
		ctx:       ctx,
		cancel:    cancel,
		closeFunc: cf,
		closed:    make(chan struct{}),
	}

	go c.closeOnContextDone()
	return c
}

func (c *contextCloser) Context() context.Context {
	return c.ctx
}

func (c *contextCloser) Children() *sync.WaitGroup {
	return &c.children
}

func (c *contextCloser) AddCloserChild(child ContextCloser) {
	c.children.Add(1)
	go func(parent, child ContextCloser) {
		<-parent.Closing()       // wait until parent is closing
		child.Close()            // signal child to close
		parent.Children().Done() // child signals it is done
	}(c, child)
}

// Close is the external close function. it's a wrapper around internalClose
// that waits on Closed()
func (c *contextCloser) Close() error {
	c.internalClose()
	<-c.Closed() // wait until we're totally done.
	return c.closeErr
}

func (c *contextCloser) Closing() <-chan struct{} {
	return c.Context().Done()
}

func (c *contextCloser) Closed() <-chan struct{} {
	return c.closed
}

func (c *contextCloser) internalClose() {
	go c.closeOnce.Do(c.closeLogic)
}

// the _actual_ close process.
func (c *contextCloser) closeLogic() {
	// this function should only be called once (hence the sync.Once).
	// and it will panic at the bottom (on close(c.closed)) otherwise.

	c.cancel()                 // signal that we're shutting down (Closing)
	c.closeErr = c.closeFunc() // actually run the close logic
	c.children.Wait()          // wait till all children are done.
	close(c.closed)            // signal that we're shut down (Closed)
}

// if parent context is shut down before we call Close explicitly,
// we need to go through the Close motions anyway. Hence all the sync
// stuff all over the place...
func (c *contextCloser) closeOnContextDone() {
	c.Children().Add(1)  // we're a child goroutine, to be waited upon.
	<-c.Context().Done() // wait until parent (context) is done.
	c.internalClose()
	c.Children().Done()
}
