// +build ignore

// WARNING: this implementation is not correct.
// here only for historical purposes.

package goprocess

import (
	"sync"
)

// process implements Process
type process struct {
	children  sync.WaitGroup // wait group for child goroutines
	teardown  TeardownFunc   // called to run the teardown logic.
	closing   chan struct{}  // closed once close starts.
	closed    chan struct{}  // closed once close is done.
	closeOnce sync.Once      // ensure close is only called once.
	closeErr  error          // error to return to clients of Close()
}

// newProcess constructs and returns a Process.
// It will call tf TeardownFunc exactly once:
//  **after** all children have fully Closed,
//  **after** entering <-Closing(), and
//  **before** <-Closed().
func newProcess(tf TeardownFunc) *process {
	if tf == nil {
		tf = nilTeardownFunc
	}

	return &process{
		teardown: tf,
		closed:   make(chan struct{}),
		closing:  make(chan struct{}),
	}
}

func (p *process) WaitFor(q Process) {
	p.children.Add(1) // p waits on q to be done
	go func(p *process, q Process) {
		<-q.Closed()      // wait until q is closed
		p.children.Done() // p done waiting on q
	}(p, q)
}

func (p *process) AddChildNoWait(child Process) {
	go func(p, child Process) {
		<-p.Closing() // wait until p is closing
		child.Close() // close child
	}(p, child)
}

func (p *process) AddChild(child Process) {
	select {
	case <-p.Closing():
		panic("attempt to add child to closing or closed process")
	default:
	}

	p.children.Add(1) // p waits on child to be done
	go func(p *process, child Process) {
		<-p.Closing()     // wait until p is closing
		child.Close()     // close child and wait
		p.children.Done() // p done waiting on child
	}(p, child)
}

func (p *process) Go(f ProcessFunc) Process {
	select {
	case <-p.Closing():
		panic("attempt to add child to closing or closed process")
	default:
	}

	// this is very similar to AddChild, but also runs the func
	// in the child. we replicate it here to save one goroutine.
	child := newProcessGoroutines(nil)
	child.children.Add(1) // child waits on func to be done
	p.AddChild(child)
	go func() {
		f(child)
		child.children.Done() // wait on child's children to be done.
		child.Close()         // close to tear down.
	}()
	return child
}

// Close is the external close function.
// it's a wrapper around internalClose that waits on Closed()
func (p *process) Close() error {
	p.closeOnce.Do(p.doClose)
	<-p.Closed() // sync.Once should block, but this checks chan is closed too
	return p.closeErr
}

func (p *process) Closing() <-chan struct{} {
	return p.closing
}

func (p *process) Closed() <-chan struct{} {
	return p.closed
}

// the _actual_ close process.
func (p *process) doClose() {
	// this function should only be called once (hence the sync.Once).
	// and it will panic (on closing channels) otherwise.

	close(p.closing)          // signal that we're shutting down (Closing)
	p.children.Wait()         // wait till all children are done (before teardown)
	p.closeErr = p.teardown() // actually run the close logic (ok safe to teardown)
	close(p.closed)           // signal that we're shut down (Closed)
}
