package core

import (
	"testing"

	peer "github.com/jbenet/go-ipfs/peer"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

func TestSubsetWhenMaxIsGreaterThanLengthOfSlice(t *testing.T) {
	var ps []peer.PeerInfo
	sizeofSlice := 100
	for i := 0; i < sizeofSlice; i++ {
		pid, err := testutil.RandPeerID()
		if err != nil {
			t.Fatal(err)
		}

		ps = append(ps, peer.PeerInfo{ID: pid})
	}
	out := randomSubsetOfPeers(ps, 2*sizeofSlice)
	if len(out) != len(ps) {
		t.Fail()
	}
}
