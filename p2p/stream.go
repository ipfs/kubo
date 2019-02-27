package p2p

import (
	"io"
	"sync"

	ma "gx/ipfs/QmTZBfrPJmjWsCvHEtX5FE6KimVJhsJg5sBbqEFYf4UZtL/go-multiaddr"
	ifconnmgr "gx/ipfs/QmXa6sgzUvP5bgF5CyyV36bZYv5VDRwttggQYUPvFybLVd/go-libp2p-interface-connmgr"
	net "gx/ipfs/QmY3ArotKMKaL7YGfbQfyDrib6RVraLqZYWXZvVgZktBxp/go-libp2p-net"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	manet "gx/ipfs/Qmc85NSvmSG4Frn9Vb2cBc1rMyULH6D3TNVEfCzSKoUpip/go-multiaddr-net"
)

const cmgrTag = "stream-fwd"

// Stream holds information on active incoming and outgoing p2p streams.
type Stream struct {
	id uint64

	Protocol protocol.ID

	OriginAddr ma.Multiaddr
	TargetAddr ma.Multiaddr
	peer       peer.ID

	Local  manet.Conn
	Remote net.Stream

	Registry *StreamRegistry
}

// close stream endpoints and deregister it
func (s *Stream) close() error {
	s.Registry.Close(s)
	return nil
}

// reset closes stream endpoints and deregisters it
func (s *Stream) reset() error {
	s.Registry.Reset(s)
	return nil
}

func (s *Stream) startStreaming() {
	go func() {
		_, err := io.Copy(s.Local, s.Remote)
		if err != nil {
			s.reset()
		} else {
			s.close()
		}
	}()

	go func() {
		_, err := io.Copy(s.Remote, s.Local)
		if err != nil {
			s.reset()
		} else {
			s.close()
		}
	}()
}

// StreamRegistry is a collection of active incoming and outgoing proto app streams.
type StreamRegistry struct {
	sync.Mutex

	Streams map[uint64]*Stream
	conns   map[peer.ID]int
	nextID  uint64

	ifconnmgr.ConnManager
}

// Register registers a stream to the registry
func (r *StreamRegistry) Register(streamInfo *Stream) {
	r.Lock()
	defer r.Unlock()

	r.ConnManager.TagPeer(streamInfo.peer, cmgrTag, 20)
	r.conns[streamInfo.peer]++

	streamInfo.id = r.nextID
	r.Streams[r.nextID] = streamInfo
	r.nextID++

	streamInfo.startStreaming()
}

// Deregister deregisters stream from the registry
func (r *StreamRegistry) Deregister(streamID uint64) {
	r.Lock()
	defer r.Unlock()

	s, ok := r.Streams[streamID]
	if !ok {
		return
	}
	p := s.peer
	r.conns[p]--
	if r.conns[p] < 1 {
		delete(r.conns, p)
		r.ConnManager.UntagPeer(p, cmgrTag)
	}

	delete(r.Streams, streamID)
}

// Close stream endpoints and deregister it
func (r *StreamRegistry) Close(s *Stream) error {
	s.Local.Close()
	s.Remote.Close()
	s.Registry.Deregister(s.id)
	return nil
}

// Reset closes stream endpoints and deregisters it
func (r *StreamRegistry) Reset(s *Stream) error {
	s.Local.Close()
	s.Remote.Reset()
	s.Registry.Deregister(s.id)
	return nil
}
