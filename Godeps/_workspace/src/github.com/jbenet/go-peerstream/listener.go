package peerstream

import (
	"errors"
	"fmt"
	"net"
	"sync"

	tec "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-temp-err-catcher"
)

// AcceptConcurrency is how many connections can simultaneously be
// in process of being accepted. Handshakes can sometimes occur as
// part of this process, so it may take some time. It is imporant to
// rate limit lest a malicious influx of connections would cause our
// node to consume all its resources accepting new connections.
var AcceptConcurrency = 200

type Listener struct {
	netList net.Listener
	groups  groupSet
	swarm   *Swarm

	acceptErr chan error
}

func newListener(nl net.Listener, s *Swarm) *Listener {
	return &Listener{
		netList:   nl,
		swarm:     s,
		acceptErr: make(chan error, 10),
	}
}

// String returns a string representation of the Listener
func (l *Listener) String() string {
	f := "<peerstream.Listener %s>"
	return fmt.Sprintf(f, l.netList.Addr())
}

// NetListener is the underlying net.Listener
func (l *Listener) NetListener() net.Listener {
	return l.netList
}

// Groups returns the groups this Listener belongs to
func (l *Listener) Groups() []Group {
	return l.groups.Groups()
}

// InGroup returns whether this Listener belongs to a Group
func (l *Listener) InGroup(g Group) bool {
	return l.groups.Has(g)
}

// AddGroup assigns given Group to Listener
func (l *Listener) AddGroup(g Group) {
	l.groups.Add(g)
}

// ListenersWithGroup narrows down a set of listeners to those in given group.
func ListenersWithGroup(g Group, ls []*Listener) []*Listener {
	var out []*Listener
	for _, l := range ls {
		if l.InGroup(g) {
			out = append(out, l)
		}
	}
	return out
}

// accept continously accepts incoming connections and
// adds them to the listener's Swarm. is is meant to be
// run in a goroutine.
// TODO: add rate limiting
func (l *Listener) accept() {
	var wg sync.WaitGroup
	defer func() {
		wg.Wait() // must happen before teardown
		l.teardown()
	}()

	// catching the error here is odd. doing what net/http does:
	// http://golang.org/src/net/http/server.go?s=51504:51550#L1728
	// Using the lib: https://godoc.org/github.com/jbenet/go-temp-err-catcher
	var catcher tec.TempErrCatcher

	// rate limit concurrency
	limit := make(chan struct{}, AcceptConcurrency)

	// loop forever accepting connections
	for {
		conn, err := l.netList.Accept()
		if err != nil {
			if catcher.IsTemporary(err) {
				continue
			}
			l.acceptErr <- fmt.Errorf("peerstream listener failed: %s", err)
			return // ok, problems. bail.
		}

		// add conn to swarm and listen for incoming streams
		// do this in a goroutine to avoid blocking the Accept loop.
		// note that this does not rate limit accepts.
		limit <- struct{}{} // sema down
		wg.Add(1)
		go func(conn net.Conn) {
			defer func() { <-limit }() // sema up
			defer wg.Done()

			conn2, err := l.swarm.addConn(conn, true)
			if err != nil {
				l.acceptErr <- err
				return
			}
			conn2.groups.AddSet(&l.groups) // add out groups
		}(conn)
	}
}

// AcceptError returns the error that we **might** on listener close
func (l *Listener) AcceptErrors() <-chan error {
	return l.acceptErr
}

func (l *Listener) teardown() {
	// in case we exit from network errors (accept fails) but
	// (a) client doesn't call Close, and (b) listener remains open)
	l.netList.Close()

	close(l.acceptErr)

	// remove self from swarm
	l.swarm.listenerLock.Lock()
	delete(l.swarm.listeners, l)
	l.swarm.listenerLock.Unlock()
}

func (l *Listener) Close() error {
	return l.netList.Close()
}

// addListener is the internal version of AddListener.
func (s *Swarm) addListener(nl net.Listener) (*Listener, error) {
	if nl == nil {
		return nil, errors.New("nil listener")
	}

	s.listenerLock.Lock()
	defer s.listenerLock.Unlock()

	// first, check if we already have it...
	for l := range s.listeners {
		if l.netList == nl {
			return l, nil
		}
	}

	l := newListener(nl, s)
	s.listeners[l] = struct{}{}
	go l.accept()
	return l, nil
}
