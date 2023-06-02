package bootstrap

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/test"
)

func TestRandomizeAddressList(t *testing.T) {
	var ps []peer.AddrInfo
	sizeofSlice := 10
	for i := 0; i < sizeofSlice; i++ {
		pid, err := test.RandPeerID()
		if err != nil {
			t.Fatal(err)
		}

		ps = append(ps, peer.AddrInfo{ID: pid})
	}
	out := randomizeList(ps)
	if len(out) != len(ps) {
		t.Fail()
	}
}
