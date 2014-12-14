// package swarm implements a connection muxer with a pair of channels
// to synchronize all network communication.
package swarm

import (
	conn "github.com/jbenet/go-ipfs/net/conn"
	peer "github.com/jbenet/go-ipfs/peer"
	eventlog "github.com/jbenet/go-ipfs/util/eventlog"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	ps "github.com/jbenet/go-peerstream"
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

func (s *Swarm) Close() error {
	return s.cg.Close()
}

func (s *Swarm) StreamSwarm() *ps.Swarm {
	return s.swarm
}

// Connections returns a slice of all connections.
func (s *Swarm) Connections() []conn.Conn {
	conns1 := s.swarm.Conns()
	conns2 := make([]conn.Conn, len(conns1))
	for i, c1 := range conns1 {
		conns2[i] = UnwrapConn(c1)
	}
	return conns2
}

// CloseConnection removes a given peer from swarm + closes the connection
func (s *Swarm) CloseConnection(p peer.Peer) error {
	conns := s.swarm.ConnsWithGroup(p) // boom.
	for _, c := range conns {
		c.Close()
	}
	return nil
}

// GetPeerList returns a copy of the set of peers swarm is connected to.
func (s *Swarm) GetPeerList() []peer.Peer {
	conns := s.swarm.Conns()

	seen := make(map[peer.Peer]struct{})
	peers := make([]peer.Peer, 0, len(conns))
	for _, c := range conns {
		c2 := UnwrapConn(c)
		p := c2.RemotePeer()
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

func UnwrapConn(c *ps.Conn) conn.Conn {
	return c.NetConn().(conn.Conn)
}
