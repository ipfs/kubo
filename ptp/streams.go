package ptp

import (
	"io"

	ma "gx/ipfs/QmcyqRMCAXVtYPS4DiBrA7sezL9rRGfW8Ctx7cywL4TXJj/go-multiaddr"
	peer "gx/ipfs/QmdS9KpbDyPrieswibZhkod1oXqRwZJrUPzxCofAMWpFGq/go-libp2p-peer"
)

// StreamInfo holds information on active incoming and outgoing p2p streams.
type StreamInfo struct {
	HandlerID uint64

	Protocol string

	LocalPeer peer.ID
	LocalAddr ma.Multiaddr

	RemotePeer peer.ID
	RemoteAddr ma.Multiaddr

	Local  io.ReadWriteCloser
	Remote io.ReadWriteCloser

	Registry *StreamRegistry
}

// Close closes stream endpoints and deregisters it
func (c *StreamInfo) Close() error {
	c.Local.Close()
	c.Remote.Close()
	c.Registry.Deregister(c.HandlerID)
	return nil
}

// StreamRegistry is a collection of active incoming and outgoing protocol app streams.
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
