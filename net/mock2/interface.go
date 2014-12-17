// Package mocknet provides a mock net.Network to test with.
//
// - a Mocknet has many inet.Networks
// - a Mocknet has many Links
// - a Link joins two inet.Networks
// - inet.Conns and inet.Streams are created by inet.Networks
package mocknet

import (
	"time"

	inet "github.com/jbenet/go-ipfs/net"
	peer "github.com/jbenet/go-ipfs/peer"
)

type Mocknet interface {
	GenPeer() (inet.Network, error)
	AddPeer(peer.ID) (inet.Network, error)

	// retrieve things
	Peer(peer.ID) peer.Peer
	Peers() []peer.Peer
	Net(peer.ID) inet.Network
	Nets() []inet.Network
	LinksBetweenPeers(a, b peer.Peer) []Link
	LinksBetweenNets(a, b inet.Network) []Link

	// Links are the **ability to connect**.
	// think of Links as the physical medium.
	// For p1 and p2 to connect, a link must exist between them.
	// (this makes it possible to test dial failures, and
	// things like relaying traffic)
	LinkPeers(peer.Peer, peer.Peer) (Link, error)
	LinkNets(inet.Network, inet.Network) (Link, error)
	Unlink(Link) error
	UnlinkPeers(peer.Peer, peer.Peer) error
	UnlinkNets(inet.Network, inet.Network) error

	// LinkDefaults are the default options that govern links
	// if they do not have thier own option set.
	SetLinkDefaults(LinkOptions)
	LinkDefaults() LinkOptions

	// Connections are the usual. Connecting means Dialing.
	// For convenience, if no link exists, Connect will add one.
	// (this is because Connect is called manually by tests).
	ConnectPeers(peer.Peer, peer.Peer) error
	ConnectNets(inet.Network, inet.Network) error
	DisconnectPeers(peer.Peer, peer.Peer) error
	DisconnectNets(inet.Network, inet.Network) error
}

type LinkOptions struct {
	Latency   time.Duration
	Bandwidth int // in bytes-per-second
	// we can make these values distributions down the road.
}

type Link interface {
	Networks() []inet.Network
	Peers() []peer.Peer

	SetOptions(LinkOptions)
	Options() LinkOptions

	// Metrics() Metrics
}
