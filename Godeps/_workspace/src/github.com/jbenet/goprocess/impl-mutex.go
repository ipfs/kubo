package goprocess

import (
	"sync"
)

// process implements Process
type process struct {
	children []Process     // process to close with us
	waitfors []Process     // process to only wait for
	teardown TeardownFunc  // called to run the teardown logic.
	closing  chan struct{} // closed once close starts.
	closed   chan struct{} // closed once close is done.
	closeErr error         // error to return to clients of Close()

	sync.Mutex
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
	p.Lock()

	select {
	case <-p.Closed():
		panic("Process cannot wait after being closed")
	default:
	}

	p.waitfors = append(p.waitfors, q)
	p.Unlock()
}

func (p *process) AddChildNoWait(child Process) {
	p.Lock()

	select {
	case <-p.Closed():
		panic("Process cannot add children after being closed")
	default:
	}

	p.children = append(p.children, child)
	p.Unlock()
}

func (p *process) AddChild(child Process) {
	p.Lock()

	select {
	case <-p.Closed():
		panic("Process cannot add children after being closed")
	default:
	}

	p.waitfors = append(p.waitfors, child)
	p.children = append(p.children, child)
	p.Unlock()
}

func (p *process) Go(f ProcessFunc) {
	child := newProcess(nil)
	p.AddChild(child)
	go func() {
		f(child)
		child.Close() // close to tear down.
	}()
}

// Close is the external close function.
// it's a wrapper around internalClose that waits on Closed()
func (p *process) Close() error {
	p.Lock()
	defer p.Unlock()

	// if already closed, get out.
	select {
	case <-p.Closed():
		return p.closeErr
	default:
	}

	p.doClose()
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
	// this function is only be called once (protected by p.Lock()).
	// and it will panic (on closing channels) otherwise.

	close(p.closing) // signal that we're shutting down (Closing)

	for _, c := range p.children {
		go c.Close() // force all children to shut down
	}

	for _, w := range p.waitfors {
		<-w.Closed() // wait till all waitfors are fully closed (before teardown)
	}

	p.closeErr = p.teardown() // actually run the close logic (ok safe to teardown)
	close(p.closed)           // signal that we're shut down (Closed)
}
