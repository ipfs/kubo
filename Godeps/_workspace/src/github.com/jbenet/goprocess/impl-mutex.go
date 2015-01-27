package goprocess

import (
	"sync"
)

// process implements Process
type process struct {
	children []Process     // process to close with us
	waitfors []Process     // process to only wait for
	teardown TeardownFunc  // called to run the teardown logic.
	waiting  chan struct{} // closed when CloseAfterChildrenClosed is called.
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
	if q == nil {
		panic("waiting for nil process")
	}

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
	if child == nil {
		panic("adding nil child process")
	}

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
	if child == nil {
		panic("adding nil child process")
	}

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

func (p *process) Go(f ProcessFunc) Process {
	child := newProcess(nil)
	p.AddChild(child)

	waitFor := newProcess(nil)
	child.WaitFor(waitFor) // prevent child from closing
	go func() {
		f(child)
		waitFor.Close()            // allow child to close.
		child.CloseAfterChildren() // close to tear down.
	}()
	return child
}

// Close is the external close function.
// it's a wrapper around internalClose that waits on Closed()
func (p *process) Close() error {
	p.Lock()

	// if already closing, or closed, get out. (but wait!)
	select {
	case <-p.Closing():
		p.Unlock()
		<-p.Closed()
		return p.closeErr
	default:
	}

	p.doClose()
	p.Unlock()
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

	for len(p.children) > 0 || len(p.waitfors) > 0 {
		for _, c := range p.children {
			go c.Close() // force all children to shut down
		}
		p.children = nil // clear them

		// we must be careful not to iterate over waitfors directly, as it may
		// change under our feet.
		wf := p.waitfors
		p.waitfors = nil // clear them
		for _, w := range wf {
			// Here, we wait UNLOCKED, so that waitfors who are in the middle of
			// adding a child to us can finish. we will immediately close the child.
			p.Unlock()
			<-w.Closed() // wait till all waitfors are fully closed (before teardown)
			p.Lock()
		}
	}

	p.closeErr = p.teardown() // actually run the close logic (ok safe to teardown)
	close(p.closed)           // signal that we're shut down (Closed)
}

// We will only wait on the children we have now.
// We will not wait on children added subsequently.
// this may change in the future.
func (p *process) CloseAfterChildren() error {
	p.Lock()
	select {
	case <-p.Closed():
		p.Unlock()
		return p.Close() // get error. safe, after p.Closed()
	case <-p.waiting: // already called it.
		p.Unlock()
		<-p.Closed()
		return p.Close() // get error. safe, after p.Closed()
	default:
	}
	p.Unlock()

	// here only from one goroutine.

	nextToWaitFor := func() Process {
		p.Lock()
		defer p.Unlock()
		for _, e := range p.waitfors {
			select {
			case <-e.Closed():
			default:
				return e
			}
		}
		return nil
	}

	// wait for all processes we're waiting for are closed.
	// the semantics here are simple: we will _only_ close
	// if there are no processes currently waiting for.
	for next := nextToWaitFor(); next != nil; next = nextToWaitFor() {
		<-next.Closed()
	}

	// YAY! we're done. close
	return p.Close()
}
