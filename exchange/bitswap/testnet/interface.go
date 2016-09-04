package bitswap

import (
	bsnet "github.com/ipfs/go-ipfs/exchange/bitswap/network"
	peer "gx/ipfs/QmWXjJo15p4pzT7cayEwZi2sWgJqLnGDof6ZGMh9xBgU1p/go-libp2p-peer"
	"gx/ipfs/QmYpVUnnedgGrp6cX2pBii5HRQgcSr778FiKVe7o7nF5Z3/go-testutil"
)

type Network interface {
	Adapter(testutil.Identity) bsnet.BitSwapNetwork

	HasPeer(peer.ID) bool
}
