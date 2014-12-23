// Package net provides an interface for ipfs to interact with the network through
package net

import (
	ic "github.com/jbenet/go-ipfs/crypto"
	swarm "github.com/jbenet/go-ipfs/net/swarm"
	peer "github.com/jbenet/go-ipfs/peer"

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

func (c *conn_) NewStreamWithProtocol(pr ProtocolID) (Stream, error) {
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

func (c *conn_) LocalMultiaddr() ma.Multiaddr {
	return c.SwarmConn().LocalMultiaddr()
}

func (c *conn_) RemoteMultiaddr() ma.Multiaddr {
	return c.SwarmConn().RemoteMultiaddr()
}

func (c *conn_) LocalPeer() peer.ID {
	return c.SwarmConn().LocalPeer()
}

func (c *conn_) RemotePeer() peer.ID {
	return c.SwarmConn().RemotePeer()
}

func (c *conn_) LocalPrivateKey() ic.PrivKey {
	return c.SwarmConn().LocalPrivateKey()
}

func (c *conn_) RemotePublicKey() ic.PubKey {
	return c.SwarmConn().RemotePublicKey()
}

// network implements the Network interface,
type network struct {
	local peer.ID      // local peer
	mux   Mux          // protocol multiplexing
	swarm *swarm.Swarm // peer connection multiplexing
	ps    peer.Peerstore
	ids   *IDService

	cg ctxgroup.ContextGroup // for Context closing
}

// NewNetwork constructs a new network and starts listening on given addresses.
func NewNetwork(ctx context.Context, listen []ma.Multiaddr, local peer.ID,
	peers peer.Peerstore) (Network, error) {

	s, err := swarm.NewSwarm(ctx, listen, local, peers)
	if err != nil {
		return nil, err
	}

	n := &network{
		local: local,
		swarm: s,
		mux:   Mux{Handlers: StreamHandlerMap{}},
		cg:    ctxgroup.WithContext(ctx),
		ps:    peers,
	}

	n.cg.SetTeardown(n.close)
	n.cg.AddChildGroup(s.CtxGroup())

	s.SetStreamHandler(func(s *swarm.Stream) {
		n.mux.Handle((*stream)(s))
	})

	// setup a conn handler that immediately "asks the other side about them"
	// this is ProtocolIdentify.
	n.ids = NewIDService(n)
	s.SetConnHandler(n.newConnHandler)

	return n, nil
}

func (n *network) newConnHandler(c *swarm.Conn) {
	cc := (*conn_)(c)
	n.ids.IdentifyConn(cc)
}

// DialPeer attempts to establish a connection to a given peer.
// Respects the context.
func (n *network) DialPeer(ctx context.Context, p peer.ID) error {
	sc, err := n.swarm.Dial(ctx, p)
	if err != nil {
		return err
	}

	// identify the connection before returning.
	n.ids.IdentifyConn((*conn_)(sc))
	log.Debugf("network for %s finished dialing %s", n.local, p)
	return nil
}

func (n *network) Protocols() []ProtocolID {
	return n.mux.Protocols()
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
func (n *network) LocalPeer() peer.ID {
	return n.swarm.LocalPeer()
}

// Peers returns the connected peers
func (n *network) Peers() []peer.ID {
	return n.swarm.Peers()
}

// Peers returns the connected peers
func (n *network) Peerstore() peer.Peerstore {
	return n.ps
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

// ConnsToPeer returns the connections in this Netowrk for given peer.
func (n *network) ConnsToPeer(p peer.ID) []Conn {
	conns1 := n.swarm.ConnectionsToPeer(p)
	out := make([]Conn, len(conns1))
	for i, c := range conns1 {
		out[i] = (*conn_)(c)
	}
	return out
}

// ClosePeer connection to peer
func (n *network) ClosePeer(p peer.ID) error {
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
func (n *network) Connectedness(p peer.ID) Connectedness {
	c := n.swarm.ConnectionsToPeer(p)
	if c != nil && len(c) < 1 {
		return Connected
	}
	return NotConnected
}

// NewStream returns a new stream to given peer p.
// If there is no connection to p, attempts to create one.
// If ProtocolID is "", writes no header.
func (c *network) NewStream(pr ProtocolID, p peer.ID) (Stream, error) {
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

func (n *network) IdentifyProtocol() *IDService {
	return n.ids
}

func WriteProtocolHeader(pr ProtocolID, s Stream) error {
	if pr != "" { // only write proper protocol headers
		if err := WriteLengthPrefix(s, string(pr)); err != nil {
			return err
		}
	}
	return nil
}
