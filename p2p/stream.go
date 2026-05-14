package p2p

import (
	"io"
	"sync"

	ifconnmgr "github.com/libp2p/go-libp2p/core/connmgr"
	net "github.com/libp2p/go-libp2p/core/network"
	peer "github.com/libp2p/go-libp2p/core/peer"
	protocol "github.com/libp2p/go-libp2p/core/protocol"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
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

// close stream endpoints and deregister it.
func (s *Stream) close() {
	s.Registry.Close(s)
}

// reset closes stream endpoints and deregisters it.
func (s *Stream) reset() {
	s.Registry.Reset(s)
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

// Register registers a stream to the registry.
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

// Deregister deregisters stream from the registry.
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

// Close stream endpoints and deregister it.
func (r *StreamRegistry) Close(s *Stream) {
	_ = s.Local.Close()
	_ = s.Remote.Close()
	s.Registry.Deregister(s.id)
}

// Reset closes stream endpoints and deregisters it.
func (r *StreamRegistry) Reset(s *Stream) {
	_ = s.Local.Close()
	_ = s.Remote.Reset()
	s.Registry.Deregister(s.id)
}
