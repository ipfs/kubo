package p2p

import (
	"errors"
	"sync"

	net "gx/ipfs/QmQSbtGXCyNrj34LWL8EgXyNNYDZ8r3SwQcpW5pPxVhLnM/go-libp2p-net"
	peer "gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	p2phost "gx/ipfs/QmfH9FKYv3Jp1xiyL8sPchGBUBg6JA6XviwajAo3qgnT3B/go-libp2p-host"
)

// ListenerLocal listens for connections and proxies them to a target
type ListenerLocal interface {
	Protocol() protocol.ID
	ListenAddress() ma.Multiaddr
	TargetAddress() ma.Multiaddr

	start() error

	// Close closes the listener. Does not affect child streams
	Close() error
}

// ListenersLocal is a collection of local application proto listeners.
type ListenersLocal struct {
	sync.RWMutex

	Listeners map[string]ListenerLocal
	starting  map[string]struct{}
}

func newListenerRegistry(id peer.ID, host p2phost.Host) *ListenersLocal {
	reg := &ListenersLocal{
		Listeners: map[string]ListenerLocal{},
		starting:  map[string]struct{}{},
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
		reg.RLock()
		defer reg.RUnlock()

		for _, l := range reg.Listeners {
			if l.ListenAddress().Equal(addr) && l.Protocol() == stream.Protocol() {
				go l.(*remoteListener).handleStream(stream)
				return
			}
		}
	})

	return reg
}

// Register registers listenerInfo into this registry and starts it
func (r *ListenersLocal) Register(l ListenerLocal) error {
	r.Lock()
	k := l.ListenAddress().String()

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
func (r *ListenersLocal) Deregister(k string) (bool, error) {
	r.Lock()
	defer r.Unlock()

	if _, ok := r.starting[k]; ok {
		return false, errors.New("listener didn't start yet")
	}

	_, ok := r.Listeners[k]
	delete(r.Listeners, k)
	return ok, nil
}
