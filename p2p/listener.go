package p2p

import (
	"sync"

	"github.com/pkg/errors"
)

type Listener interface {
	Protocol() string
	ListenAddress() string
	TargetAddress() string

	// Close closes the listener. Does not affect child streams
	Close() error
}

type listenerKey struct {
	proto  string
	listen string
	target string
}

// ListenerRegistry is a collection of local application proto listeners.
type ListenerRegistry struct {
	Listeners map[listenerKey]Listener
	lk        *sync.Mutex
}

func (r *ListenerRegistry) Lock(l Listener) error {
	r.lk.Lock()

	if _, ok := r.Listeners[getListenerKey(l)]; ok {
		r.lk.Unlock()
		return errors.New("listener already registered")
	}
	return nil
}

func (r *ListenerRegistry) Unlock() {
	r.lk.Unlock()
}

// Register registers listenerInfo in this registry
func (r *ListenerRegistry) Register(l Listener) {
	defer r.lk.Unlock()

	r.Listeners[getListenerKey(l)] = l
}

// Deregister removes p2p listener from this registry
func (r *ListenerRegistry) Deregister(k listenerKey) {
	r.lk.Lock()
	defer r.lk.Unlock()

	delete(r.Listeners, k)
}

func getListenerKey(l Listener) listenerKey {
	return listenerKey{
		proto:  l.Protocol(),
		listen: l.ListenAddress(),
		target: l.TargetAddress(),
	}
}
