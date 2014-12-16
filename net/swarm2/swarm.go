// package swarm implements a connection muxer with a pair of channels
// to synchronize all network communication.
package swarm

import (
	peer "github.com/jbenet/go-ipfs/peer"
	eventlog "github.com/jbenet/go-ipfs/util/eventlog"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	ps "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-peerstream"
)

var log = eventlog.Logger("swarm2")

// Swarm is a connection muxer, allowing connections to other peers to
// be opened and closed, while still using the same Chan for all
// communication. The Chan sends/receives Messages, which note the
// destination or source Peer.
//
// Uses peerstream.Swarm
type Swarm struct {
	swarm *ps.Swarm
	local peer.Peer
	peers peer.Peerstore

	cg ctxgroup.ContextGroup
}

// NewSwarm constructs a Swarm, with a Chan.
func NewSwarm(ctx context.Context, listenAddrs []ma.Multiaddr,
	local peer.Peer, peers peer.Peerstore) (*Swarm, error) {

	// make sure our own peer is in our peerstore...
	local, err := peers.Add(local)
	if err != nil {
		return nil, err
	}

	s := &Swarm{
		swarm: ps.NewSwarm(),
		local: local,
		peers: peers,
		cg:    ctxgroup.WithContext(ctx),
	}

	// configure Swarm
	s.cg.SetTeardown(s.teardown)
	s.swarm.SetConnHandler(s.connHandler)

	return s, s.listen(listenAddrs)
}

func (s *Swarm) teardown() error {
	return s.swarm.Close()
}

// CtxGroup returns the Context Group of the swarm
func (s *Swarm) CtxGroup() ctxgroup.ContextGroup {
	return s.cg
}

// Close stops the Swarm.
func (s *Swarm) Close() error {
	return s.cg.Close()
}

// StreamSwarm returns the underlying peerstream.Swarm
func (s *Swarm) StreamSwarm() *ps.Swarm {
	return s.swarm
}

// SetStreamHandler assigns the handler for new streams.
// See peerstream.
func (s *Swarm) SetStreamHandler(handler StreamHandler) {
	s.swarm.SetStreamHandler(func(s *ps.Stream) {
		handler(wrapStream(s))
	})
}

// NewStreamWithPeer creates a new stream on any available connection to p
func (s *Swarm) NewStreamWithPeer(p peer.Peer) (*Stream, error) {
	// make sure we use OUR peers. (the tests mess with you...)
	p, err := s.peers.Add(p)
	if err != nil {
		return nil, err
	}

	// if we have no connections, try connecting.
	if len(s.ConnectionsToPeer(p)) == 0 {
		log.Debug("Swarm: NewStreamWithPeer no connections. Attempting to connect...")
		if _, err := s.Dial(context.Background(), p); err != nil {
			return nil, err
		}
	}
	log.Debug("Swarm: NewStreamWithPeer...")

	st, err := s.swarm.NewStreamWithGroup(p)
	return wrapStream(st), err
}

// StreamsWithPeer returns all the live Streams to p
func (s *Swarm) StreamsWithPeer(p peer.Peer) []*Stream {
	// make sure we use OUR peers. (the tests mess with you...)
	if p2, err := s.peers.Add(p); err == nil {
		p = p2
	}

	return wrapStreams(ps.StreamsWithGroup(p, s.swarm.Streams()))
}

// ConnectionsToPeer returns all the live connections to p
func (s *Swarm) ConnectionsToPeer(p peer.Peer) []*Conn {
	// make sure we use OUR peers. (the tests mess with you...)
	if p2, err := s.peers.Add(p); err == nil {
		p = p2
	}
	return wrapConns(ps.ConnsWithGroup(p, s.swarm.Conns()))
}

// Connections returns a slice of all connections.
func (s *Swarm) Connections() []*Conn {
	return wrapConns(s.swarm.Conns())
}

// CloseConnection removes a given peer from swarm + closes the connection
func (s *Swarm) CloseConnection(p peer.Peer) error {
	// make sure we use OUR peers. (the tests mess with you...)
	p, err := s.peers.Add(p)
	if err != nil {
		return err
	}

	conns := s.swarm.ConnsWithGroup(p) // boom.
	for _, c := range conns {
		c.Close()
	}
	return nil
}

// Peers returns a copy of the set of peers swarm is connected to.
func (s *Swarm) Peers() []peer.Peer {
	conns := s.Connections()

	seen := make(map[peer.Peer]struct{})
	peers := make([]peer.Peer, 0, len(conns))
	for _, c := range conns {
		p := c.RemotePeer()
		if _, found := seen[p]; found {
			continue
		}

		peers = append(peers, p)
	}
	return peers
}

// LocalPeer returns the local peer swarm is associated to.
func (s *Swarm) LocalPeer() peer.Peer {
	return s.local
}
