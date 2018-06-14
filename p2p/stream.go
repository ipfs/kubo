package p2p

import (
	"io"
	"sync"

	manet "gx/ipfs/QmNqRnejxJxjRroz7buhrjfU8i3yNBLa81hFtmf2pXEffN/go-multiaddr-net"
	ma "gx/ipfs/QmUxSEGbv2nmYNnfXi7839wwQqTN3kwQeUxe8dTjZWZs7J/go-multiaddr"
	net "gx/ipfs/QmXdgNhVEgjLxjUoMs5ViQL7pboAt3Y7V7eGHRiE4qrmTE/go-libp2p-net"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
)

// Stream holds information on active incoming and outgoing p2p streams.
type Stream struct {
	id uint64

	Protocol protocol.ID

	OriginAddr ma.Multiaddr
	TargetAddr ma.Multiaddr

	Local  manet.Conn
	Remote net.Stream

	Registry *StreamRegistry
}

// Close closes stream endpoints and deregisters it
func (s *Stream) Close() error {
	s.Local.Close()
	s.Remote.Close()
	s.Registry.Deregister(s.id)
	return nil
}

// Reset closes stream endpoints and deregisters it
func (s *Stream) Reset() error {
	s.Local.Close()
	s.Remote.Reset()
	s.Registry.Deregister(s.id)
	return nil
}

func (s *Stream) startStreaming() {
	go func() {
		io.Copy(s.Local, s.Remote)
		s.Reset()
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
	Streams map[uint64]*Stream
	lk      sync.Mutex

	nextID uint64
}

// Register registers a stream to the registry
func (r *StreamRegistry) Register(streamInfo *Stream) {
	r.lk.Lock()
	defer r.lk.Unlock()

	streamInfo.id = r.nextID
	r.Streams[r.nextID] = streamInfo
	r.nextID++
}

// Deregister deregisters stream from the registry
func (r *StreamRegistry) Deregister(streamID uint64) {
	r.lk.Lock()
	defer r.lk.Unlock()

	delete(r.Streams, streamID)
}
