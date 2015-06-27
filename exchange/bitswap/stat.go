package bitswap

import (
	key "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-blocks/key"
	"sort"
)

type Stat struct {
	ProvideBufLen   int
	Wantlist        []key.Key
	Peers           []string
	BlocksReceived  int
	DupBlksReceived int
}

func (bs *Bitswap) Stat() (*Stat, error) {
	st := new(Stat)
	st.ProvideBufLen = len(bs.newBlocks)
	st.Wantlist = bs.GetWantlist()
	bs.counterLk.Lock()
	st.BlocksReceived = bs.blocksRecvd
	st.DupBlksReceived = bs.dupBlocksRecvd
	bs.counterLk.Unlock()

	for _, p := range bs.engine.Peers() {
		st.Peers = append(st.Peers, p.Pretty())
	}
	sort.Strings(st.Peers)

	return st, nil
}
