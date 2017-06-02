package ptp

import (
	"io"

	ma "gx/ipfs/QmcyqRMCAXVtYPS4DiBrA7sezL9rRGfW8Ctx7cywL4TXJj/go-multiaddr"
	peer "gx/ipfs/QmdS9KpbDyPrieswibZhkod1oXqRwZJrUPzxCofAMWpFGq/go-libp2p-peer"
	"fmt"
)

// ListenerInfo holds information on a p2p listener.
type ListenerInfo struct {
	// Application protocol identifier.
	Protocol string

	// Node identity
	Identity peer.ID

	// Local protocol stream address.
	Address ma.Multiaddr

	// Local protocol stream listener.
	Closer io.Closer

	// Flag indicating whether we're still accepting incoming connections, or
	// whether this application listener has been shutdown.
	Running bool

	Registry *ListenerRegistry
}

// Close closes the listener. Does not affect child streams
func (c *ListenerInfo) Close() error {
	c.Closer.Close()
	err := c.Registry.Deregister(c.Protocol)
	return err
}

// ListenerRegistry is a collection of local application protocol listeners.
type ListenerRegistry struct {
	Listeners []*ListenerInfo
}

// Register registers listenerInfo in this registry
func (c *ListenerRegistry) Register(listenerInfo *ListenerInfo) {
	c.Listeners = append(c.Listeners, listenerInfo)
}

// Deregister removes p2p listener from this registry
func (c *ListenerRegistry) Deregister(proto string) error {
	foundAt := -1
	for i, a := range c.Listeners {
		if a.Protocol == proto {
			foundAt = i
			break
		}
	}

	if foundAt != -1 {
		c.Listeners = append(c.Listeners[:foundAt], c.Listeners[foundAt+1:]...)
		return nil
	}

	return fmt.Errorf("failed to deregister proto %s", proto)
}
