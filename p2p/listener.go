package p2p

import (
	"errors"
	"sync"

	net "gx/ipfs/QmPjvxTpVH8qJyQDnxnsxF9kv9jezKD1kozz1hs3fCGsNh/go-libp2p-net"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	p2phost "gx/ipfs/Qmb8T6YBBsjYsVGfrihQLfCJveczZnneSBqBKkYEBWDjge/go-libp2p-host"
	peer "gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"
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
	sync.RWMutex

	Listeners map[listenerKey]Listener
	starting  map[listenerKey]struct{}
}

func newListenerRegistry(id peer.ID, host p2phost.Host) *ListenerRegistry {
	reg := &ListenerRegistry{
		Listeners: map[listenerKey]Listener{},
		starting:  map[listenerKey]struct{}{},
	}

	addr, err := ma.NewMultiaddr(maPrefix + id.Pretty())
	if err != nil {
		panic(err)
	}

	host.SetStreamHandlerMatch("/x/", func(p string) bool {
		reg.RLock()
		defer reg.RUnlock()
		for _, l := range reg.Listeners {
			if l.ListenAddress().Equal(addr) && string(l.Protocol()) == p {
				return true
			}
		}

		return false
	}, func(stream net.Stream) {
		for _, l := range reg.Listeners {
			if l.ListenAddress().Equal(addr) && l.Protocol() == stream.Protocol() {
				l.(*remoteListener).handleStream(stream)
			}
		}

		// panic?
	})

	return reg
}

// Register registers listenerInfo into this registry and starts it
func (r *ListenerRegistry) Register(l Listener) error {
	r.Lock()
	k := getListenerKey(l)

	if _, ok := r.Listeners[k]; ok {
		r.Unlock()
		return errors.New("listener already registered")
	}

	r.Listeners[k] = l
	r.starting[k] = struct{}{}

	r.Unlock()

	err := l.start()

	r.Lock()
	defer r.Unlock()

	delete(r.starting, k)

	if err != nil {
		delete(r.Listeners, k)
		return err
	}

	return nil
}

// Deregister removes p2p listener from this registry
func (r *ListenerRegistry) Deregister(k listenerKey) (bool, error) {
	r.Lock()
	defer r.Unlock()

	if _, ok := r.starting[k]; ok {
		return false, errors.New("listener didn't start yet")
	}

	_, ok := r.Listeners[k]
	delete(r.Listeners, k)
	return ok, nil
}

func getListenerKey(l Listener) listenerKey {
	return listenerKey{
		proto:  string(l.Protocol()),
		listen: l.ListenAddress().String(),
		target: l.TargetAddress().String(),
	}
}
