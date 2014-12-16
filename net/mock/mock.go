// Package mocknet provides a mock net.Network to test with.
package mocknet

import (
	"fmt"
	"io"
	"sync"

	inet "github.com/jbenet/go-ipfs/net"
	peer "github.com/jbenet/go-ipfs/peer"
	eventlog "github.com/jbenet/go-ipfs/util/eventlog"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

var log = eventlog.Logger("mocknet")

type Stream struct {
	io.Reader
	io.Writer
	conn *Conn
}

func (s *Stream) Close() error {
	s.conn.removeStream(s)
	if r, ok := (s.Reader).(io.Closer); ok {
		r.Close()
	}
	if w, ok := (s.Writer).(io.Closer); ok {
		return w.Close()
	}
	return nil
}

func (s *Stream) Conn() inet.Conn {
	return s.conn
}

// wire pipe between two network conns. yay io.
func newStreamPair(n1 *Network, p2 peer.Peer) (*Stream, *Stream) {
	p1 := n1.local
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()

	s1 := &Stream{Reader: r1, Writer: w2}
	s2 := &Stream{Reader: r2, Writer: w1}

	n1.Lock()
	n1.conns[p2].addStream(s1)
	n2 := n1.conns[p2].remote
	n1.Unlock()

	n2.Lock()
	n2.conns[p1].addStream(s2)
	n2.Unlock()
	n2.handle(s2)

	return s1, s2
}

type Conn struct {
	connected bool
	local     *Network
	remote    *Network
	streams   []*Stream
	sync.RWMutex
}

func (c *Conn) Close() error {
	c.Lock()
	defer c.Unlock()

	c.connected = false
	for _, s := range c.streams {
		go s.Close()
	}
	c.streams = nil
	return nil
}

func (c *Conn) addStream(s *Stream) {
	c.Lock()
	defer c.Unlock()

	s.conn = c
	c.streams = append(c.streams, s)
}

func (c *Conn) removeStream(s *Stream) {
	c.Lock()
	defer c.Unlock()

	strs := make([]*Stream, 0, len(c.streams))
	for _, s2 := range c.streams {
		if s2 != s {
			strs = append(strs, s2)
		}
	}
}

func (c *Conn) NewStreamWithProtocol(pr inet.ProtocolID, p peer.Peer) (inet.Stream, error) {

	if _, connected := c.local.conns[p]; !connected {
		return nil, fmt.Errorf("cannot create new stream for %s. not connected.", p)
	}

	log.Debugf("NewStreamWithProtocol: %s --> %s", c.local, p)
	ss, _ := newStreamPair(c.local, p)

	if err := inet.WriteProtocolHeader(pr, ss); err != nil {
		ss.Close()
		return nil, err
	}

	return ss, nil
}

// LocalMultiaddr is the Multiaddr on this side
func (c *Conn) LocalMultiaddr() ma.Multiaddr {
	return nil
}

// LocalPeer is the Peer on our side of the connection
func (c *Conn) LocalPeer() peer.Peer {
	return c.local.local
}

// RemoteMultiaddr is the Multiaddr on the remote side
func (c *Conn) RemoteMultiaddr() ma.Multiaddr {
	return nil
}

// RemotePeer is the Peer on the remote side
func (c *Conn) RemotePeer() peer.Peer {
	return c.remote.local
}

// network implements the Network interface,
type Network struct {
	local peer.Peer // local peer
	mux   inet.Mux  // protocol multiplexing

	conns map[peer.Peer]*Conn
	sync.RWMutex

	cg ctxgroup.ContextGroup // for Context closing
}

func MakeNetworks(ctx context.Context, peers []peer.Peer) (nets []*Network, err error) {
	nets = make([]*Network, len(peers))
	for i, p := range peers {
		ps := peer.NewPeerstore()
		nets[i], err = newNetwork(ctx, p, ps)
		if err != nil {
			return nil, err
		}
	}

	i := 0
	for _, n1 := range nets {
		for _, n2 := range nets {
			n1.conns[n2.local] = &Conn{local: n1, remote: n2}
			log.Debugf("%d setup %s -> %s", i, n1, n2)
			i++
		}
	}

	return nets, nil
}

// NewNetwork constructs a new Mock network
func newNetwork(ctx context.Context, local peer.Peer, peers peer.Peerstore) (*Network, error) {

	n := &Network{
		local: local,
		mux:   inet.Mux{Handlers: inet.StreamHandlerMap{}},
		cg:    ctxgroup.WithContext(ctx),
		conns: map[peer.Peer]*Conn{},
	}

	n.cg.SetTeardown(n.close)
	return n, nil
}
func (n *Network) String() string {
	return fmt.Sprintf("<Network %s - %d conns>", n.local, len(n.conns))
}

func (n *Network) handle(s inet.Stream) {
	go n.mux.Handle(s)
}

// DialPeer attempts to establish a connection to a given peer.
// Respects the context.
func (n *Network) DialPeer(ctx context.Context, p peer.Peer) error {
	n.Lock()
	defer n.Unlock()

	c, ok := n.conns[p]
	if !ok {
		return fmt.Errorf("cannot connect to %s (mock needs all nets at start)", p)
	}
	c.connected = true
	return nil
}

// CtxGroup returns the network's ContextGroup
func (n *Network) CtxGroup() ctxgroup.ContextGroup {
	return n.cg
}

// LocalPeer the network's LocalPeer
func (n *Network) LocalPeer() peer.Peer {
	return n.local
}

// Peers returns the connected peers
func (n *Network) Peers() []peer.Peer {
	n.RLock()
	defer n.RUnlock()

	peers := make([]peer.Peer, 0, len(n.conns))
	for _, c := range n.conns {
		if c.connected {
			peers = append(peers, c.RemotePeer())
		}
	}
	return peers
}

// Conns returns the connected peers
func (n *Network) Conns() []inet.Conn {
	n.RLock()
	defer n.RUnlock()

	out := make([]inet.Conn, 0, len(n.conns))
	for _, c := range n.conns {
		if c.connected {
			out = append(out, c)
		}
	}
	return out
}

// ClosePeer connection to peer
func (n *Network) ClosePeer(p peer.Peer) error {
	c, ok := n.conns[p]
	if !ok {
		return nil
	}
	return c.Close()
}

// close is the real teardown function
func (n *Network) close() error {
	for _, c := range n.conns {
		c.Close()
	}
	return nil
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
	return []ma.Multiaddr{}
}

// InterfaceListenAddresses returns a list of addresses at which this network
// listens. It expands "any interface" addresses (/ip4/0.0.0.0, /ip6/::) to
// use the known local interfaces.
func (n *Network) InterfaceListenAddresses() ([]ma.Multiaddr, error) {
	return []ma.Multiaddr{}, nil
}

// Connectedness returns a state signaling connection capabilities
// For now only returns Connecter || NotConnected. Expand into more later.
func (n *Network) Connectedness(p peer.Peer) inet.Connectedness {
	n.Lock()
	defer n.Unlock()

	if _, found := n.conns[p]; found && n.conns[p].connected {
		return inet.Connected
	}
	return inet.NotConnected
}

// NewStream returns a new stream to given peer p.
// If there is no connection to p, attempts to create one.
// If ProtocolID is "", writes no header.
func (c *Network) NewStream(pr inet.ProtocolID, p peer.Peer) (inet.Stream, error) {
	return c.conns[p].NewStreamWithProtocol(pr, p)
}

// SetHandler sets the protocol handler on the Network's Muxer.
// This operation is threadsafe.
func (n *Network) SetHandler(p inet.ProtocolID, h inet.StreamHandler) {
	n.mux.SetHandler(p, h)
}
