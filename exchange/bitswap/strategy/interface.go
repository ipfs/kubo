package strategy

import (
	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

type Strategy interface {
	// Returns a slice of Peers that
	Peers() []*peer.Peer

	// WantList returns the WantList for the given Peer
	BlockIsWantedByPeer(u.Key, *peer.Peer) bool

	// ShouldSendTo(Peer) decides whether to send data to this Peer
	ShouldSendBlockToPeer(u.Key, *peer.Peer) bool

	// Seed initializes the decider to a deterministic state
	Seed(int64)

	// MessageReceived records receipt of message for accounting purposes
	MessageReceived(*peer.Peer, bsmsg.BitSwapMessage) error

	// MessageSent records sending of message for accounting purposes
	MessageSent(*peer.Peer, bsmsg.BitSwapMessage) error

	NumBytesSentTo(*peer.Peer) uint64

	NumBytesReceivedFrom(*peer.Peer) uint64
}

type WantList interface {
	// Peer returns the owner of the WantList
	Peer() *peer.Peer

	// Intersection returns the keys common to both WantLists
	Intersection(WantList) WantList

	KeySet
}

// TODO(brian): potentially move this somewhere more generic. For now, it's
// useful in BitSwap operations.

type KeySet interface {
	Contains(u.Key) bool
	Keys() []u.Key
}
