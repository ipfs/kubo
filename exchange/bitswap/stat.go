package bitswap

import (
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	u "github.com/jbenet/go-ipfs/util"
)

type Stat struct {
	ProvideBufLen int
	Wantlist      []u.Key
	Peers         []peer.ID
}

func (bs *Bitswap) Stat() (*Stat, error) {
	st := new(Stat)
	st.ProvideBufLen = len(bs.newBlocks)
	st.Wantlist = bs.GetWantlist()

	st.Peers = bs.engine.Peers()

	return st, nil
}
