package host

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	eventlog "github.com/jbenet/go-ipfs/util/eventlog"

	inet "github.com/jbenet/go-ipfs/p2p/net2"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	protocol "github.com/jbenet/go-ipfs/p2p/protocol"
)

var log = eventlog.Logger("p2p/host")

// Host is an object participating in a p2p network, which
// implements protocols or provides services. It handles
// requests like a Server, and issues requests like a Client.
// It is called Host because it is both Server and Client (and Peer
// may be confusing).
type Host interface {
	// ID returns the (local) peer.ID associated with this Host
	ID() peer.ID

	// Peerstore returns the Host's repository of Peer Addresses and Keys.
	Peerstore() peer.Peerstore

	// Networks returns the Network interface of the Host
	Network() inet.Network

	// Mux returns the Mux multiplexing incoming streams to protocol handlers
	Mux() *protocol.Mux

	// Connect ensures there is a connection between this host and the peer with
	// given peer.ID. Connect will absorb the addresses in pi into its internal
	// peerstore. If there is not an active connection, Connect will issue a
	// h.Network.Dial, and block until a connection is open, or an error is
	// returned. // TODO: Relay + NAT.
	Connect(ctx context.Context, pi peer.PeerInfo) error

	// SetStreamHandler sets the protocol handler on the Host's Mux.
	// This is equivalent to:
	//   host.Mux().SetHandler(proto, handler)
	// (Threadsafe)
	SetStreamHandler(pid protocol.ID, handler inet.StreamHandler)

	// NewStream opens a new stream to given peer p, and writes a p2p/protocol
	// header with given protocol.ID. If there is no connection to p, attempts
	// to create one. If ProtocolID is "", writes no header.
	// (Threadsafe)
	NewStream(pid protocol.ID, p peer.ID) (inet.Stream, error)

	// Close shuts down the host, its Network, and services.
	Close() error
}
