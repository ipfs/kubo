package network

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// Adapter provides network connectivity for BitSwap sessions
type Adapter interface {

	// DialPeer ensures there is a connection to peer.
	DialPeer(*peer.Peer) error

	// SendMessage sends a BitSwap message to a peer.
	SendMessage(
		context.Context,
		*peer.Peer,
		bsmsg.BitSwapMessage) error

	// SendRequest sends a BitSwap message to a peer and waits for a response.
	SendRequest(
		context.Context,
		*peer.Peer,
		bsmsg.BitSwapMessage) (incoming bsmsg.BitSwapMessage, err error)

	// SetDelegate registers the Reciver to handle messages received from the
	// network.
	SetDelegate(Receiver)
}

type Receiver interface {
	ReceiveMessage(
		ctx context.Context, sender *peer.Peer, incoming bsmsg.BitSwapMessage) (
		destination *peer.Peer, outgoing bsmsg.BitSwapMessage)

	ReceiveError(error)
}

// TODO rename -> Router?
type Routing interface {
	// FindProvidersAsync returns a channel of providers for the given key
	FindProvidersAsync(context.Context, u.Key, int) <-chan *peer.Peer

	// Provide provides the key to the network
	Provide(context.Context, u.Key) error
}
