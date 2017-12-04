package bitswap

import (
	"sort"

	cid "gx/ipfs/QmeSrf6pzut73u6zLQkRFQ3ygt3k6XFT2kjdYP8Tnkwwyg/go-cid"
)

type Stat struct {
	ProvideBufLen   int
	Wantlist        []*cid.Cid
	Peers           []string
	BlocksReceived  uint64
	DataReceived    uint64
	BlocksSent      uint64
	DataSent        uint64
	DupBlksReceived uint64
	DupDataReceived uint64
}

func (bs *Bitswap) Stat() (*Stat, error) {
	st := new(Stat)
	st.ProvideBufLen = len(bs.newBlocks)
	st.Wantlist = bs.GetWantlist()
	bs.counterLk.Lock()
	c := bs.counters
	st.BlocksReceived = c.blocksRecvd
	st.DupBlksReceived = c.dupBlocksRecvd
	st.DupDataReceived = c.dupDataRecvd
	st.BlocksSent = c.blocksSent
	st.DataSent = c.dataSent
	st.DataReceived = c.dataRecvd
	bs.counterLk.Unlock()

	peers := bs.engine.Peers()
	st.Peers = make([]string, 0, len(peers))

	for _, p := range peers {
		st.Peers = append(st.Peers, p.Pretty())
	}
	sort.Strings(st.Peers)

	return st, nil
}
