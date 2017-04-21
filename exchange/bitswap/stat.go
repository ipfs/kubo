package bitswap

import (
	"sort"

	cid "gx/ipfs/QmYhQaCYEcaPPjxJX7YcPcVKkQfRy6sJ7B3XmGFk82XYdQ/go-cid"
)

type Stat struct {
	ProvideBufLen   int
	Wantlist        []*cid.Cid
	Peers           []string
	BlocksReceived  int
	DataReceived    uint64
	BlocksSent      int
	DataSent        uint64
	DupBlksReceived int
	DupDataReceived uint64
}

func (bs *Bitswap) Stat() (*Stat, error) {
	st := new(Stat)
	st.ProvideBufLen = len(bs.newBlocks)
	st.Wantlist = bs.GetWantlist()
	bs.counterLk.Lock()
	st.BlocksReceived = bs.blocksRecvd
	st.DupBlksReceived = bs.dupBlocksRecvd
	st.DupDataReceived = bs.dupDataRecvd
	st.BlocksSent = bs.blocksSent
	st.DataSent = bs.dataSent
	st.DataReceived = bs.dataRecvd
	bs.counterLk.Unlock()

	for _, p := range bs.engine.Peers() {
		st.Peers = append(st.Peers, p.Pretty())
	}
	sort.Strings(st.Peers)

	return st, nil
}
