package p2p

import (
	"errors"
	"sync"

	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
)

// Listener listens for connections and proxies them to a target
type Listener interface {
	Protocol() protocol.ID
	ListenAddress() ma.Multiaddr
	TargetAddress() ma.Multiaddr

	start() error

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
	sync.Mutex

	Listeners map[listenerKey]Listener
}

// Register registers listenerInfo into this registry and starts it
func (r *ListenerRegistry) Register(l Listener) error {
	r.Lock()

	if _, ok := r.Listeners[getListenerKey(l)]; ok {
		r.Unlock()
		return errors.New("listener already registered")
	}

	r.Listeners[getListenerKey(l)] = l

	r.Unlock()

	if err := l.start(); err != nil {
		r.Lock()
		defer r.Lock()

		delete(r.Listeners, getListenerKey(l))
		return err
	}

	return nil
}

// Deregister removes p2p listener from this registry
func (r *ListenerRegistry) Deregister(k listenerKey) bool {
	r.Lock()
	defer r.Unlock()

	_, ok := r.Listeners[k]
	delete(r.Listeners, k)
	return ok
}

func getListenerKey(l Listener) listenerKey {
	return listenerKey{
		proto:  string(l.Protocol()),
		listen: l.ListenAddress().String(),
		target: l.TargetAddress().String(),
	}
}
