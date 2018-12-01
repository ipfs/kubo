package p2p

import (
	"errors"
	"sync"

	ma "gx/ipfs/QmRKLtwMw131aK7ugC3G7ybpumMz78YrJe5dzneyindvG1/go-multiaddr"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	p2phost "gx/ipfs/QmahxMNoNuSsgQefo9rkpcfRFmQrMN6Q99aztKXf63K7YJ/go-libp2p-host"
	net "gx/ipfs/QmenvQQy4bFGSiHJUGupVmCRHfetg5rH3vTp9Z2f6v2KXR/go-libp2p-net"
)

// Listener listens for connections and proxies them to a target
type Listener interface {
	Protocol() protocol.ID
	ListenAddress() ma.Multiaddr
	TargetAddress() ma.Multiaddr

	key() string

	// close closes the listener. Does not affect child streams
	close()
}

// Listeners manages a group of Listener implementations,
// checking for conflicts and optionally dispatching connections
type Listeners struct {
	sync.RWMutex

	Listeners map[string]Listener
}

func newListenersLocal() *Listeners {
	return &Listeners{
		Listeners: map[string]Listener{},
	}
}

func newListenersP2P(host p2phost.Host) *Listeners {
	reg := &Listeners{
		Listeners: map[string]Listener{},
	}

	host.SetStreamHandlerMatch("/x/", func(p string) bool {
		reg.RLock()
		defer reg.RUnlock()

		_, ok := reg.Listeners[p]
		return ok
	}, func(stream net.Stream) {
		reg.RLock()
		defer reg.RUnlock()

		l := reg.Listeners[string(stream.Protocol())]
		if l != nil {
			go l.(*remoteListener).handleStream(stream)
		}
	})

	return reg
}

// Register registers listenerInfo into this registry and starts it
func (r *Listeners) Register(l Listener) error {
	r.Lock()
	defer r.Unlock()

	if _, ok := r.Listeners[l.key()]; ok {
		return errors.New("listener already registered")
	}

	r.Listeners[l.key()] = l
	return nil
}

func (r *Listeners) Close(matchFunc func(listener Listener) bool) int {
	todo := make([]Listener, 0)
	r.Lock()
	for _, l := range r.Listeners {
		if !matchFunc(l) {
			continue
		}

		if _, ok := r.Listeners[l.key()]; ok {
			delete(r.Listeners, l.key())
			todo = append(todo, l)
		}
	}
	r.Unlock()

	for _, l := range todo {
		l.close()
	}

	return len(todo)
}
