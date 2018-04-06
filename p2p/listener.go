package p2p

import (
	pstore "gx/ipfs/QmZb7hAgQEhW9dBbzBudU39gCeD4zbe6xafD52LUuF4cUN/go-libp2p-peerstore"
	peer "gx/ipfs/QmcJukH2sAFjY3HdBKq35WDzWoL3UUu2gt9wdfqZTUyM74/go-libp2p-peer"
	p2phost "gx/ipfs/QmdHyfNVTZ5VtUx4Xz23z8wtnioSrFQ28XSfpVkdhQBkGA/go-libp2p-host"
)

type Listener interface {
	Protocol() string
	Address() string

	// Close closes the listener. Does not affect child streams
	Close() error
}

// NewP2P creates new P2P struct
func NewP2P(identity peer.ID, peerHost p2phost.Host, peerstore pstore.Peerstore) *P2P {
	return &P2P{
		identity:  identity,
		peerHost:  peerHost,
		peerstore: peerstore,
	}
}

// ListenerRegistry is a collection of local application proto listeners.
type ListenerRegistry struct {
	Listeners map[string]Listener
}

// Register registers listenerInfo2 in this registry
func (c *ListenerRegistry) Register(listenerInfo Listener) {
	c.Listeners[listenerInfo.Protocol()] = listenerInfo
}

// Deregister removes p2p listener from this registry
func (c *ListenerRegistry) Deregister(proto string) {
	delete(c.Listeners, proto)
}
