// package net provides an interface for ipfs to interact with the network through
package net

import (
	msg "github.com/jbenet/go-ipfs/net/message"
	mux "github.com/jbenet/go-ipfs/net/mux"
	swarm "github.com/jbenet/go-ipfs/net/swarm"
	peer "github.com/jbenet/go-ipfs/peer"
	util "github.com/jbenet/go-ipfs/util"
	ctxc "github.com/jbenet/go-ipfs/util/ctxcloser"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

// IpfsNetwork implements the Network interface,
type IpfsNetwork struct {

	// local peer
	local peer.Peer

	// protocol multiplexing
	muxer *mux.Muxer

	// peer connection multiplexing
	swarm *swarm.Swarm

	// network context closer
	ctxc.ContextCloser
}

// NewIpfsNetwork is the structure that implements the network interface
func NewIpfsNetwork(ctx context.Context, listen []ma.Multiaddr, local peer.Peer,
	peers peer.Peerstore, pmap *mux.ProtocolMap) (*IpfsNetwork, error) {

	in := &IpfsNetwork{
		local:         local,
		muxer:         mux.NewMuxer(ctx, *pmap),
		ContextCloser: ctxc.NewContextCloser(ctx, nil),
	}

	var err error
	in.swarm, err = swarm.NewSwarm(ctx, listen, local, peers)
	if err != nil {
		in.Close()
		return nil, err
	}

	in.AddCloserChild(in.swarm)
	in.AddCloserChild(in.muxer)

	// remember to wire components together.
	in.muxer.Pipe.ConnectTo(in.swarm.Pipe)

	return in, nil
}

// Listen handles incoming connections on given Multiaddr.
// func (n *IpfsNetwork) Listen(*ma.Muliaddr) error {}

// DialPeer attempts to establish a connection to a given peer.
// Respects the context.
func (n *IpfsNetwork) DialPeer(ctx context.Context, p peer.Peer) error {
	err := util.ContextDo(ctx, func() error {
		_, err := n.swarm.Dial(p)
		return err
	})
	return err
}

// ClosePeer connection to peer
func (n *IpfsNetwork) ClosePeer(p peer.Peer) error {
	return n.swarm.CloseConnection(p)
}

// IsConnected returns whether a connection to given peer exists.
func (n *IpfsNetwork) IsConnected(p peer.Peer) (bool, error) {
	return n.swarm.GetConnection(p.ID()) != nil, nil
}

// GetProtocols returns the protocols registered in the network.
func (n *IpfsNetwork) GetProtocols() *mux.ProtocolMap {
	// copy over because this map should be read only.
	pmap := mux.ProtocolMap{}
	for id, proto := range n.muxer.Protocols {
		pmap[id] = proto
	}
	return &pmap
}

// SendMessage sends given Message out
func (n *IpfsNetwork) SendMessage(m msg.NetMessage) error {
	n.swarm.Outgoing <- m
	return nil
}

// GetPeerList returns the networks list of connected peers
func (n *IpfsNetwork) GetPeerList() []peer.Peer {
	return n.swarm.GetPeerList()
}

// GetBandwidthTotals returns the total amount of bandwidth transferred
func (n *IpfsNetwork) GetBandwidthTotals() (in uint64, out uint64) {
	return n.muxer.GetBandwidthTotals()
}

// ListenAddresses returns a list of addresses at which this network listens.
func (n *IpfsNetwork) ListenAddresses() []ma.Multiaddr {
	return n.swarm.ListenAddresses()
}

// InterfaceListenAddresses returns a list of addresses at which this network
// listens. It expands "any interface" addresses (/ip4/0.0.0.0, /ip6/::) to
// use the known local interfaces.
func (n *IpfsNetwork) InterfaceListenAddresses() ([]ma.Multiaddr, error) {
	return n.swarm.InterfaceListenAddresses()
}
