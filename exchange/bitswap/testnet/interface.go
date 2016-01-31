package bitswap

import (
	bsnet "github.com/ipfs/go-ipfs/exchange/bitswap/network"
	"github.com/ipfs/go-ipfs/util/testutil"
	peer "gx/ipfs/QmQQCBoWhMZtStYuAAo2uDNNLit9n7yX5ANBecfjKq4XBn/go-libp2p/p2p/peer"
)

type Network interface {
	Adapter(testutil.Identity) bsnet.BitSwapNetwork

	HasPeer(peer.ID) bool
}
