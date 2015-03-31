package bitswap

import (
	u "github.com/ipfs/go-ipfs/util"
	"sort"
)

type Stat struct {
	ProvideBufLen int
	Wantlist      []u.Key
	Peers         []string
}

func (bs *Bitswap) Stat() (*Stat, error) {
	st := new(Stat)
	st.ProvideBufLen = len(bs.newBlocks)
	st.Wantlist = bs.GetWantlist()

	for _, p := range bs.engine.Peers() {
		st.Peers = append(st.Peers, p.Pretty())
	}
	sort.Strings(st.Peers)

	return st, nil
}
