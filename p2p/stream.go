package p2p

import (
	"io"
	"sync"

	manet "gx/ipfs/QmV6FjemM1K8oXjrvuq3wuVWWoU2TLDPmNnKrxHzY3v6Ai/go-multiaddr-net"
	ifconnmgr "gx/ipfs/QmV9T3rTezwJhMJQBzVrGj2tncJwv23dTKdxzXrUxAvWFi/go-libp2p-interface-connmgr"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	peer "gx/ipfs/QmbNepETomvmXfz1X5pHNFD2QuPqnqi47dTd94QJWSorQ3/go-libp2p-peer"
	net "gx/ipfs/QmfDPh144WGBqRxZb1TGDHerbMnZATrHZggAPw7putNnBq/go-libp2p-net"
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
