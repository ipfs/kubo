package swarm

import (
	"fmt"

	peer "github.com/jbenet/go-ipfs/p2p/peer"

	inet "github.com/jbenet/go-ipfs/p2p/net"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

// Network implements the inet.Network interface.
// It is simply a swarm, with a few different functions
// to implement inet.Network.
type Network Swarm

// NewNetwork constructs a new network and starts listening on given addresses.
func NewNetwork(ctx context.Context, listen []ma.Multiaddr, local peer.ID,
	peers peer.Peerstore) (*Network, error) {

	s, err := NewSwarm(ctx, listen, local, peers)
	if err != nil {
		return nil, err
	}

	return (*Network)(s), nil
}

// DialPeer attempts to establish a connection to a given peer.
// Respects the context.
func (n *Network) DialPeer(ctx context.Context, p peer.ID) (inet.Conn, error) {
	log.Debugf("[%s] network dialing peer [%s]", n.local, p)
	sc, err := n.Swarm().Dial(ctx, p)
	if err != nil {
		return nil, err
	}

	log.Debugf("network for %s finished dialing %s", n.local, p)
	return inet.Conn(sc), nil
}

// CtxGroup returns the network's ContextGroup
func (n *Network) CtxGroup() ctxgroup.ContextGroup {
	return n.cg
}

// Swarm returns the network's peerstream.Swarm
func (n *Network) Swarm() *Swarm {
	return (*Swarm)(n)
}

// LocalPeer the network's LocalPeer
func (n *Network) LocalPeer() peer.ID {
	return n.Swarm().LocalPeer()
}

// Peers returns the connected peers
func (n *Network) Peers() []peer.ID {
	return n.Swarm().Peers()
}

// Peers returns the connected peers
func (n *Network) Peerstore() peer.Peerstore {
	return n.Swarm().peers
}

// Conns returns the connected peers
func (n *Network) Conns() []inet.Conn {
	conns1 := n.Swarm().Connections()
	out := make([]inet.Conn, len(conns1))
	for i, c := range conns1 {
		out[i] = inet.Conn(c)
	}
	return out
}

// ConnsToPeer returns the connections in this Netowrk for given peer.
func (n *Network) ConnsToPeer(p peer.ID) []inet.Conn {
	conns1 := n.Swarm().ConnectionsToPeer(p)
	out := make([]inet.Conn, len(conns1))
	for i, c := range conns1 {
		out[i] = inet.Conn(c)
	}
	return out
}

// ClosePeer connection to peer
func (n *Network) ClosePeer(p peer.ID) error {
	return n.Swarm().CloseConnection(p)
}

// close is the real teardown function
func (n *Network) close() error {
	return n.Swarm().Close()
}

// Close calls the ContextCloser func
func (n *Network) Close() error {
	return n.Swarm().cg.Close()
}

// Listen tells the network to start listening on given multiaddrs.
func (n *Network) Listen(addrs ...ma.Multiaddr) error {
	return n.Swarm().Listen(addrs...)
}

// ListenAddresses returns a list of addresses at which this network listens.
func (n *Network) ListenAddresses() []ma.Multiaddr {
	return n.Swarm().ListenAddresses()
}

// InterfaceListenAddresses returns a list of addresses at which this network
// listens. It expands "any interface" addresses (/ip4/0.0.0.0, /ip6/::) to
// use the known local interfaces.
func (n *Network) InterfaceListenAddresses() ([]ma.Multiaddr, error) {
	return n.Swarm().InterfaceListenAddresses()
}

// Connectedness returns a state signaling connection capabilities
// For now only returns Connected || NotConnected. Expand into more later.
func (n *Network) Connectedness(p peer.ID) inet.Connectedness {
	c := n.Swarm().ConnectionsToPeer(p)
	if c != nil && len(c) > 0 {
		return inet.Connected
	}
	return inet.NotConnected
}

// NewStream returns a new stream to given peer p.
// If there is no connection to p, attempts to create one.
func (n *Network) NewStream(p peer.ID) (inet.Stream, error) {
	log.Debugf("[%s] network opening stream to peer [%s]", n.local, p)
	s, err := n.Swarm().NewStreamWithPeer(p)
	if err != nil {
		return nil, err
	}

	return inet.Stream(s), nil
}

// SetHandler sets the protocol handler on the Network's Muxer.
// This operation is threadsafe.
func (n *Network) SetStreamHandler(h inet.StreamHandler) {
	n.Swarm().SetStreamHandler(h)
}

// SetConnHandler sets the conn handler on the Network.
// This operation is threadsafe.
func (n *Network) SetConnHandler(h inet.ConnHandler) {
	n.Swarm().SetConnHandler(func(c *Conn) {
		h(inet.Conn(c))
	})
}

// String returns a string representation of Network.
func (n *Network) String() string {
	return fmt.Sprintf("<Network %s>", n.LocalPeer())
}

// Notify signs up Notifiee to receive signals when events happen
func (n *Network) Notify(f inet.Notifiee) {
	n.Swarm().Notify(f)
}

// StopNotify unregisters Notifiee fromr receiving signals
func (n *Network) StopNotify(f inet.Notifiee) {
	n.Swarm().StopNotify(f)
}
