// package ctxgroup provides the ContextGroup, a hybrid between the
// context.Context and sync.WaitGroup, which models process trees.
package ctxgroup

import (
	"sync"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

// TeardownFunc is a function used to cleanup state at the end of the
// lifecycle of a process.
type TeardownFunc func() error

// ChildFunc is a function to register as a child. It will be automatically
// tracked.
type ChildFunc func(parent ContextGroup)

var nilTeardownFunc = func() error { return nil }

// ContextGroup is an interface for services able to be opened and closed.
// It has a parent Context, and Children. But ContextGroup is not a proper
// "tree" like the Context tree. It is more like a Context-WaitGroup hybrid.
// It models a main object with a few children objects -- and, unlike the
// context -- concerns itself with the parent-child closing semantics:
//
// - Can define an optional TeardownFunc (func() error) to be run at Close time.
// - Children call Children().Add(1) to be waited upon
// - Children can select on <-Closing() to know when they should shut down.
// - Close() will wait until all children call Children().Done()
// - <-Closed() signals when the service is completely closed.
//
// ContextGroup can be embedded into the main object itself. In that case,
// the teardownFunc (if a member function) has to be set after the struct
// is intialized:
//
//  type service struct {
//  	ContextGroup
//  	net.Conn
//  }
//
//  func (s *service) close() error {
//  	return s.Conn.Close()
//  }
//
//  func newService(ctx context.Context, c net.Conn) *service {
//  	s := &service{c}
//  	s.ContextGroup = NewContextGroup(ctx, s.close)
//  	return s
//  }
//
type ContextGroup interface {

	// Context is the context of this ContextGroup. It is "sort of" a parent.
	Context() context.Context

	// SetTeardown assigns the teardown function.
	// It is called exactly _once_ when the ContextGroup is Closed.
	SetTeardown(tf TeardownFunc)

	// Children is a sync.Waitgroup for all children goroutines that should
	// shut down completely before this service is said to be "closed".
	// Follows the semantics of WaitGroup:
	//
	//  Children().Add(1) // add one more dependent child
	//  Children().Done() // child signals it is done
	//
	// WARNING: this is deprecated and will go away soon.
	Children() *sync.WaitGroup

	// AddChildGroup registers a dependent ContextGroup child. The child will
	// be closed when this parent is closed, and waited upon to finish. It is
	// the functional equivalent of the following:
	//
	//	parent.Children().Add(1) // add one more dependent child
	//  go func(parent, child ContextGroup) {
	//  	<-parent.Closing()       // wait until parent is closing
	//  	child.Close()            // signal child to close
	//  	parent.Children().Done() // child signals it is done
	//	}(a, b)
	//
	AddChildGroup(c ContextGroup)

	// AddChildFunc registers a dependent ChildFund. The child will receive
	// its parent ContextGroup, and can wait on its signals. Child references
	// tracked automatically. It equivalent to the following:
	//
	//  go func(parent, child ContextGroup) {
	//
	//  	<-parent.Closing()       // wait until parent is closing
	//  	child.Close()            // signal child to close
	//  	parent.Children().Done() // child signals it is done
	//	}(a, b)
	//
	AddChildFunc(c ChildFunc)

	// Close is a method to call when you wish to stop this ContextGroup
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

// contextGroup is a Closer with a cancellable context
type contextGroup struct {
	ctx    context.Context
	cancel context.CancelFunc

	// called to run the teardown logic.
	teardownFunc TeardownFunc

	// closed is released once the close function is done.
	closed chan struct{}

	// wait group for child goroutines
	children sync.WaitGroup

	// sync primitive to ensure the close logic is only called once.
	closeOnce sync.Once

	// error to return to clients of Close().
	closeErr error
}

// newContextGroup constructs and returns a ContextGroup. It will call
// cf TeardownFunc before its Done() Wait signals fire.
func newContextGroup(ctx context.Context, cf TeardownFunc) ContextGroup {
	ctx, cancel := context.WithCancel(ctx)
	c := &contextGroup{
		ctx:    ctx,
		cancel: cancel,
		closed: make(chan struct{}),
	}
	c.SetTeardown(cf)

	c.Children().Add(1) // initialize with 1. calling Close will decrement it.
	go c.closeOnContextDone()
	return c
}

// SetTeardown assigns the teardown function.
func (c *contextGroup) SetTeardown(cf TeardownFunc) {
	if cf == nil {
		cf = nilTeardownFunc
	}
	c.teardownFunc = cf
}

func (c *contextGroup) Context() context.Context {
	return c.ctx
}

func (c *contextGroup) Children() *sync.WaitGroup {
	return &c.children
}

func (c *contextGroup) AddChildGroup(child ContextGroup) {
	c.children.Add(1)
	go func(parent, child ContextGroup) {
		<-parent.Closing()       // wait until parent is closing
		child.Close()            // signal child to close
		parent.Children().Done() // child signals it is done
	}(c, child)
}

func (c *contextGroup) AddChildFunc(child ChildFunc) {
	c.children.Add(1)
	go func(parent ContextGroup, child ChildFunc) {
		child(parent)
		parent.Children().Done() // child signals it is done
	}(c, child)
}

// Close is the external close function. it's a wrapper around internalClose
// that waits on Closed()
func (c *contextGroup) Close() error {
	c.internalClose()
	<-c.Closed() // wait until we're totally done.
	return c.closeErr
}

func (c *contextGroup) Closing() <-chan struct{} {
	return c.Context().Done()
}

func (c *contextGroup) Closed() <-chan struct{} {
	return c.closed
}

func (c *contextGroup) internalClose() {
	go c.closeOnce.Do(c.closeLogic)
}

// the _actual_ close process.
func (c *contextGroup) closeLogic() {
	// this function should only be called once (hence the sync.Once).
	// and it will panic at the bottom (on close(c.closed)) otherwise.

	c.cancel()                    // signal that we're shutting down (Closing)
	c.closeErr = c.teardownFunc() // actually run the close logic
	c.children.Wait()             // wait till all children are done.
	close(c.closed)               // signal that we're shut down (Closed)
}

// if parent context is shut down before we call Close explicitly,
// we need to go through the Close motions anyway. Hence all the sync
// stuff all over the place...
func (c *contextGroup) closeOnContextDone() {
	<-c.Context().Done() // wait until parent (context) is done.
	c.internalClose()
	c.Children().Done()
}

// WithTeardown constructs and returns a ContextGroup with
// cf TeardownFunc (and context.Background)
func WithTeardown(cf TeardownFunc) ContextGroup {
	if cf == nil {
		panic("nil TeardownFunc")
	}
	return newContextGroup(context.Background(), cf)
}

// WithContext constructs and returns a ContextGroup with given context
func WithContext(ctx context.Context) ContextGroup {
	if ctx == nil {
		panic("nil Context")
	}
	return newContextGroup(ctx, nil)
}

// WithContextAndTeardown constructs and returns a ContextGroup with
// cf TeardownFunc (and context.Background)
func WithContextAndTeardown(ctx context.Context, cf TeardownFunc) ContextGroup {
	if ctx == nil {
		panic("nil Context")
	}
	if cf == nil {
		panic("nil TeardownFunc")
	}
	return newContextGroup(ctx, cf)
}

// WithParent constructs and returns a ContextGroup with given parent
func WithParent(p ContextGroup) ContextGroup {
	if p == nil {
		panic("nil ContextGroup")
	}
	c := newContextGroup(p.Context(), nil)
	p.AddChildGroup(c)
	return c
}

// WithBackground returns a ContextGroup with context.Background()
func WithBackground() ContextGroup {
	return newContextGroup(context.Background(), nil)
}
