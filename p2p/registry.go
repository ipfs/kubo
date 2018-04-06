package p2p

import (
	"fmt"
	"io"

	peer "gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	manet "gx/ipfs/QmV6FjemM1K8oXjrvuq3wuVWWoU2TLDPmNnKrxHzY3v6Ai/go-multiaddr-net"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	net "gx/ipfs/QmZNJyx9GGCX4GeuHnLB8fxaxMLs4MjTjHokxfQcCd6Nve/go-libp2p-net"
)

// ListenerRegistry is a collection of local application proto listeners.
type ListenerRegistry struct {
	Listeners []Listener
}

// Register registers listenerInfo2 in this registry
func (c *ListenerRegistry) Register(listenerInfo Listener) {
	c.Listeners = append(c.Listeners, listenerInfo)
}

// Deregister removes p2p listener from this registry
func (c *ListenerRegistry) Deregister(proto string) error {
	foundAt := -1
	for i, a := range c.Listeners {
		if a.Protocol() == proto {
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

// StreamInfo holds information on active incoming and outgoing p2p streams.
type StreamInfo struct {
	HandlerID uint64

	Protocol string

	LocalPeer peer.ID
	LocalAddr ma.Multiaddr

	RemotePeer peer.ID
	RemoteAddr ma.Multiaddr

	Local  manet.Conn
	Remote net.Stream

	Registry *StreamRegistry
}

// Close closes stream endpoints and deregisters it
func (s *StreamInfo) Close() error {
	s.Local.Close()
	s.Remote.Close()
	s.Registry.Deregister(s.HandlerID)
	return nil
}

// Reset closes stream endpoints and deregisters it
func (s *StreamInfo) Reset() error {
	s.Local.Close()
	s.Remote.Reset()
	s.Registry.Deregister(s.HandlerID)
	return nil
}

func (s *StreamInfo) startStreaming() {
	go func() {
		_, err := io.Copy(s.Local, s.Remote)
		if err != nil {
			s.Reset()
		} else {
			s.Close()
		}
	}()

	go func() {
		_, err := io.Copy(s.Remote, s.Local)
		if err != nil {
			s.Reset()
		} else {
			s.Close()
		}
	}()
}

// StreamRegistry is a collection of active incoming and outgoing proto app streams.
type StreamRegistry struct {
	Streams []*StreamInfo

	nextID uint64
}

// Register registers a stream to the registry
func (c *StreamRegistry) Register(streamInfo *StreamInfo) {
	streamInfo.HandlerID = c.nextID
	c.Streams = append(c.Streams, streamInfo)
	c.nextID++
}

// Deregister deregisters stream from the registry
func (c *StreamRegistry) Deregister(handlerID uint64) {
	foundAt := -1
	for i, s := range c.Streams {
		if s.HandlerID == handlerID {
			foundAt = i
			break
		}
	}

	if foundAt != -1 {
		c.Streams = append(c.Streams[:foundAt], c.Streams[foundAt+1:]...)
	}
}
