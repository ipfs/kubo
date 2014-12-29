// Package net provides an interface for ipfs to interact with the network through
package net

import (
	"fmt"

	ic "github.com/jbenet/go-ipfs/crypto"
	peer "github.com/jbenet/go-ipfs/p2p/peer"

	inet "github.com/jbenet/go-ipfs/net"
	ids "github.com/jbenet/go-ipfs/net/services/identify"
	mux "github.com/jbenet/go-ipfs/net/services/mux"
	relay "github.com/jbenet/go-ipfs/net/services/relay"
	swarm "github.com/jbenet/go-ipfs/net/swarm"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	eventlog "github.com/jbenet/go-ipfs/util/eventlog"
)

var log = eventlog.Logger("net/mux")

type stream swarm.Stream

func (s *stream) SwarmStream() *swarm.Stream {
	return (*swarm.Stream)(s)
}

// Conn returns the connection this stream is part of.
func (s *stream) Conn() inet.Conn {
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

func (s *conn_) String() string {
	return s.SwarmConn().String()
}

func (c *conn_) SwarmConn() *swarm.Conn {
	return (*swarm.Conn)(c)
}

func (c *conn_) NewStreamWithProtocol(pr inet.ProtocolID) (inet.Stream, error) {
	s, err := (*swarm.Conn)(c).NewStream()
	if err != nil {
		return nil, err
	}

	ss := (*stream)(s)

	if err := mux.WriteProtocolHeader(pr, ss); err != nil {
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

// Network implements the inet.Network interface.
// It uses a swarm to connect to remote hosts.
type Network struct {
	local peer.ID // local peer
	ps    peer.Peerstore

	swarm *swarm.Swarm // peer connection multiplexing
	mux   mux.Mux      // protocol multiplexing
	ids   *ids.IDService
	relay *relay.RelayService

	cg ctxgroup.ContextGroup // for Context closing
}

// NewNetwork constructs a new network and starts listening on given addresses.
func NewNetwork(ctx context.Context, listen []ma.Multiaddr, local peer.ID,
	peers peer.Peerstore) (*Network, error) {

	s, err := swarm.NewSwarm(ctx, listen, local, peers)
	if err != nil {
		return nil, err
	}

	n := &Network{
		local: local,
		swarm: s,
		mux:   mux.Mux{Handlers: inet.StreamHandlerMap{}},
		cg:    ctxgroup.WithContext(ctx),
		ps:    peers,
	}

	n.cg.SetTeardown(n.close)
	n.cg.AddChildGroup(s.CtxGroup())

	s.SetStreamHandler(func(s *swarm.Stream) {
		n.mux.Handle((*stream)(s))
	})

	// setup ProtocolIdentify to immediately "asks the other side about them"
	n.ids = ids.NewIDService(n)
	s.SetConnHandler(n.newConnHandler)

	// setup ProtocolRelay to allow traffic relaying.
	// Feed things we get for ourselves into the muxer.
	n.relay = relay.NewRelayService(n.cg.Context(), n, n.mux.HandleSync)
	return n, nil
}

func (n *Network) newConnHandler(c *swarm.Conn) {
	cc := (*conn_)(c)
	n.ids.IdentifyConn(cc)
}

// DialPeer attempts to establish a connection to a given peer.
// Respects the context.
func (n *Network) DialPeer(ctx context.Context, p peer.ID) error {
	log.Debugf("[%s] network dialing peer [%s]", n.local, p)
	sc, err := n.swarm.Dial(ctx, p)
	if err != nil {
		return err
	}

	// identify the connection before returning.
	done := make(chan struct{})
	go func() {
		n.ids.IdentifyConn((*conn_)(sc))
		close(done)
	}()

	// respect don contexteone
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	log.Debugf("network for %s finished dialing %s", n.local, p)
	return nil
}

// Protocols returns the ProtocolIDs of all the registered handlers.
func (n *Network) Protocols() []inet.ProtocolID {
	return n.mux.Protocols()
}

// CtxGroup returns the network's ContextGroup
func (n *Network) CtxGroup() ctxgroup.ContextGroup {
	return n.cg
}

// Swarm returns the network's peerstream.Swarm
func (n *Network) Swarm() *swarm.Swarm {
	return n.Swarm()
}

// LocalPeer the network's LocalPeer
func (n *Network) LocalPeer() peer.ID {
	return n.swarm.LocalPeer()
}

// Peers returns the connected peers
func (n *Network) Peers() []peer.ID {
	return n.swarm.Peers()
}

// Peers returns the connected peers
func (n *Network) Peerstore() peer.Peerstore {
	return n.ps
}

// Conns returns the connected peers
func (n *Network) Conns() []inet.Conn {
	conns1 := n.swarm.Connections()
	out := make([]inet.Conn, len(conns1))
	for i, c := range conns1 {
		out[i] = (*conn_)(c)
	}
	return out
}

// ConnsToPeer returns the connections in this Netowrk for given peer.
func (n *Network) ConnsToPeer(p peer.ID) []inet.Conn {
	conns1 := n.swarm.ConnectionsToPeer(p)
	out := make([]inet.Conn, len(conns1))
	for i, c := range conns1 {
		out[i] = (*conn_)(c)
	}
	return out
}

// ClosePeer connection to peer
func (n *Network) ClosePeer(p peer.ID) error {
	return n.swarm.CloseConnection(p)
}

// close is the real teardown function
func (n *Network) close() error {
	return n.swarm.Close()
}

// Close calls the ContextCloser func
func (n *Network) Close() error {
	return n.cg.Close()
}

// BandwidthTotals returns the total amount of bandwidth transferred
func (n *Network) BandwidthTotals() (in uint64, out uint64) {
	// need to implement this. probably best to do it in swarm this time.
	// need a "metrics" object
	return 0, 0
}

// ListenAddresses returns a list of addresses at which this network listens.
func (n *Network) ListenAddresses() []ma.Multiaddr {
	return n.swarm.ListenAddresses()
}

// InterfaceListenAddresses returns a list of addresses at which this network
// listens. It expands "any interface" addresses (/ip4/0.0.0.0, /ip6/::) to
// use the known local interfaces.
func (n *Network) InterfaceListenAddresses() ([]ma.Multiaddr, error) {
	return swarm.InterfaceListenAddresses(n.swarm)
}

// Connectedness returns a state signaling connection capabilities
// For now only returns Connected || NotConnected. Expand into more later.
func (n *Network) Connectedness(p peer.ID) inet.Connectedness {
	c := n.swarm.ConnectionsToPeer(p)
	if c != nil && len(c) > 0 {
		return inet.Connected
	}
	return inet.NotConnected
}

// NewStream returns a new stream to given peer p.
// If there is no connection to p, attempts to create one.
// If ProtocolID is "", writes no header.
func (n *Network) NewStream(pr inet.ProtocolID, p peer.ID) (inet.Stream, error) {
	log.Debugf("[%s] network opening stream to peer [%s]: %s", n.local, p, pr)
	s, err := n.swarm.NewStreamWithPeer(p)
	if err != nil {
		return nil, err
	}

	ss := (*stream)(s)

	if err := mux.WriteProtocolHeader(pr, ss); err != nil {
		ss.Close()
		return nil, err
	}

	return ss, nil
}

// SetHandler sets the protocol handler on the Network's Muxer.
// This operation is threadsafe.
func (n *Network) SetHandler(p inet.ProtocolID, h inet.StreamHandler) {
	n.mux.SetHandler(p, h)
}

// String returns a string representation of Network.
func (n *Network) String() string {
	return fmt.Sprintf("<Network %s>", n.LocalPeer())
}

// IdentifyProtocol returns the network's IDService
func (n *Network) IdentifyProtocol() *ids.IDService {
	return n.ids
}
