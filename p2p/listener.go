package p2p

type Listener interface {
	Protocol() string
	Address() string

	// Close closes the listener. Does not affect child streams
	Close() error
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
