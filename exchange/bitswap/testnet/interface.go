package bitswap

import (
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	"github.com/jbenet/go-ipfs/util/testutil"
)

type Network interface {
	Adapter(testutil.Identity) bsnet.BitSwapNetwork

	HasPeer(peer.ID) bool
}
