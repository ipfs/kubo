package p2p

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
}

// Register registers listenerInfo in this registry
func (c *ListenerRegistry) Register(l Listener) {
	c.Listeners[getListenerKey(l)] = l
}

// Deregister removes p2p listener from this registry
func (c *ListenerRegistry) Deregister(k listenerKey) {
	delete(c.Listeners, k)
}

func getListenerKey(l Listener) listenerKey {
	return listenerKey{
		proto:  l.Protocol(),
		listen: l.ListenAddress(),
		target: l.TargetAddress(),
	}
}
