package bitswap

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	peer "github.com/jbenet/go-ipfs/peer"
)

type Network interface {
	Adapter(peer.Peer) bsnet.BitSwapNetwork

	HasPeer(peer.Peer) bool

	SendMessage(
		ctx context.Context,
		from peer.Peer,
		to peer.Peer,
		message bsmsg.BitSwapMessage) error

	SendRequest(
		ctx context.Context,
		from peer.Peer,
		to peer.Peer,
		message bsmsg.BitSwapMessage) (
		incoming bsmsg.BitSwapMessage, err error)
}
