package network

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// BitSwapNetwork provides network connectivity for BitSwap sessions
type BitSwapNetwork interface {

	// DialPeer ensures there is a connection to peer.
	DialPeer(context.Context, peer.ID) error

	// SendMessage sends a BitSwap message to a peer.
	SendMessage(
		context.Context,
		peer.ID,
		bsmsg.BitSwapMessage) error

	// SendRequest sends a BitSwap message to a peer and waits for a response.
	SendRequest(
		context.Context,
		peer.ID,
		bsmsg.BitSwapMessage) (incoming bsmsg.BitSwapMessage, err error)

	Peerstore() peer.Peerstore

	// SetDelegate registers the Reciver to handle messages received from the
	// network.
	SetDelegate(Receiver)

	Routing
}

// Implement Receiver to receive messages from the BitSwapNetwork
type Receiver interface {
	ReceiveMessage(
		ctx context.Context, sender peer.ID, incoming bsmsg.BitSwapMessage) (
		destination peer.ID, outgoing bsmsg.BitSwapMessage)

	ReceiveError(error)
}

type Routing interface {
	// FindProvidersAsync returns a channel of providers for the given key
	FindProvidersAsync(context.Context, u.Key, int) <-chan peer.ID

	// Provide provides the key to the network
	Provide(context.Context, u.Key) error
}
