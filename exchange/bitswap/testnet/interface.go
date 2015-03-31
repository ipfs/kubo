package bitswap

import (
	bsnet "github.com/ipfs/go-ipfs/exchange/bitswap/network"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	"github.com/ipfs/go-ipfs/util/testutil"
)

type Network interface {
	Adapter(testutil.Identity) bsnet.BitSwapNetwork

	HasPeer(peer.ID) bool
}
