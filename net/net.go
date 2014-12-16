// Package net provides an interface for ipfs to interact with the network through
package net

import (
	swarm "github.com/jbenet/go-ipfs/net/swarm2"
	peer "github.com/jbenet/go-ipfs/peer"
	util "github.com/jbenet/go-ipfs/util"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

type stream swarm.Stream

func (s *stream) SwarmStream() *swarm.Stream {
	return (*swarm.Stream)(s)
}

// Conn returns the connection this stream is part of.
func (s *stream) Conn() Conn {
	c := s.SwarmStream().Conn()
	return (*conn_)(c)
}

// Conn returns the connection this stream is part of.
func (s *stream) Close() error {
	return s.SwarmStream().Close()
}

// Read reads bytes from a stream.
func (s *stream) Read(p []byte) (n int, err error) {
	return s.SwarmStream().Read(p)
}

// Write writes bytes to a stream, flushing for each call.
func (s *stream) Write(p []byte) (n int, err error) {
	return s.SwarmStream().Write(p)
}

type conn_ swarm.Conn

func (c *conn_) SwarmConn() *swarm.Conn {
	return (*swarm.Conn)(c)
}

func (c *conn_) NewStreamWithProtocol(pr ProtocolID, p peer.Peer) (Stream, error) {
	s, err := (*swarm.Conn)(c).NewStream()
	if err != nil {
		return nil, err
	}

	ss := (*stream)(s)

	if err := WriteProtocolHeader(pr, ss); err != nil {
		ss.Close()
		return nil, err
	}

	return ss, nil
}

// LocalMultiaddr is the Multiaddr on this side
func (c *conn_) LocalMultiaddr() ma.Multiaddr {
	return c.SwarmConn().LocalMultiaddr()
}

// LocalPeer is the Peer on our side of the connection
func (c *conn_) LocalPeer() peer.Peer {
	return c.SwarmConn().LocalPeer()
}

// RemoteMultiaddr is the Multiaddr on the remote side
func (c *conn_) RemoteMultiaddr() ma.Multiaddr {
	return c.SwarmConn().RemoteMultiaddr()
}

// RemotePeer is the Peer on the remote side
func (c *conn_) RemotePeer() peer.Peer {
	return c.SwarmConn().RemotePeer()
}

// network implements the Network interface,
type network struct {
	local peer.Peer    // local peer
	mux   Mux          // protocol multiplexing
	swarm *swarm.Swarm // peer connection multiplexing

	cg ctxgroup.ContextGroup // for Context closing
}

// NewNetwork constructs a new network and starts listening on given addresses.
func NewNetwork(ctx context.Context, listen []ma.Multiaddr, local peer.Peer,
	peers peer.Peerstore) (Network, error) {

	s, err := swarm.NewSwarm(ctx, listen, local, peers)
	if err != nil {
		return nil, err
	}

	n := &network{
		local: local,
		swarm: s,
		mux:   Mux{},
		cg:    ctxgroup.WithContext(ctx),
	}

	s.SetStreamHandler(func(s *swarm.Stream) {
		n.mux.Handle((*stream)(s))
	})

	n.cg.SetTeardown(n.close)
	n.cg.AddChildGroup(s.CtxGroup())
	return n, nil
}

// DialPeer attempts to establish a connection to a given peer.
// Respects the context.
func (n *network) DialPeer(ctx context.Context, p peer.Peer) error {
	err := util.ContextDo(ctx, func() error {
		_, err := n.swarm.Dial(p)
		return err
	})
	return err
}

// CtxGroup returns the network's ContextGroup
func (n *network) CtxGroup() ctxgroup.ContextGroup {
	return n.cg
}

// Swarm returns the network's peerstream.Swarm
func (n *network) Swarm() *swarm.Swarm {
	return n.Swarm()
}

// LocalPeer the network's LocalPeer
func (n *network) LocalPeer() peer.Peer {
	return n.swarm.LocalPeer()
}

// Peers returns the connected peers
func (n *network) Peers() []peer.Peer {
	return n.swarm.Peers()
}

// Conns returns the connected peers
func (n *network) Conns() []Conn {
	conns1 := n.swarm.Connections()
	out := make([]Conn, len(conns1))
	for i, c := range conns1 {
		out[i] = (*conn_)(c)
	}
	return out
}

// ClosePeer connection to peer
func (n *network) ClosePeer(p peer.Peer) error {
	return n.swarm.CloseConnection(p)
}

// close is the real teardown function
func (n *network) close() error {
	return n.swarm.Close()
}

// Close calls the ContextCloser func
func (n *network) Close() error {
	return n.cg.Close()
}

// BandwidthTotals returns the total amount of bandwidth transferred
func (n *network) BandwidthTotals() (in uint64, out uint64) {
	// need to implement this. probably best to do it in swarm this time.
	// need a "metrics" object
	return 0, 0
}

// ListenAddresses returns a list of addresses at which this network listens.
func (n *network) ListenAddresses() []ma.Multiaddr {
	return n.swarm.ListenAddresses()
}

// InterfaceListenAddresses returns a list of addresses at which this network
// listens. It expands "any interface" addresses (/ip4/0.0.0.0, /ip6/::) to
// use the known local interfaces.
func (n *network) InterfaceListenAddresses() ([]ma.Multiaddr, error) {
	return swarm.InterfaceListenAddresses(n.swarm)
}

// Connectedness returns a state signaling connection capabilities
// For now only returns Connected || NotConnected. Expand into more later.
func (n *network) Connectedness(p peer.Peer) Connectedness {
	c := n.swarm.ConnectionsToPeer(p)
	if c != nil && len(c) < 1 {
		return Connected
	}
	return NotConnected
}

// NewStream returns a new stream to given peer p.
// If there is no connection to p, attempts to create one.
// If ProtocolID is "", writes no header.
func (c *network) NewStream(pr ProtocolID, p peer.Peer) (Stream, error) {
	s, err := c.swarm.NewStreamWithPeer(p)
	if err != nil {
		return nil, err
	}

	ss := (*stream)(s)

	if err := WriteProtocolHeader(pr, ss); err != nil {
		ss.Close()
		return nil, err
	}

	return ss, nil
}

// SetHandler sets the protocol handler on the Network's Muxer.
// This operation is threadsafe.
func (n *network) SetHandler(p ProtocolID, h StreamHandler) {
	n.mux.SetHandler(p, h)
}

func WriteProtocolHeader(pr ProtocolID, s Stream) error {
	if pr != "" { // only write proper protocol headers
		if err := WriteLengthPrefix(s, string(pr)); err != nil {
			return err
		}
	}
	return nil
}
