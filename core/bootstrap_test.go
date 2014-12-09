package core

import (
	"testing"

	peer "github.com/jbenet/go-ipfs/peer"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

func TestSubsetWhenMaxIsGreaterThanLengthOfSlice(t *testing.T) {
	var ps []peer.Peer
	sizeofSlice := 100
	for i := 0; i < sizeofSlice; i++ {
		ps = append(ps, testutil.RandPeer())
	}
	out := randomSubsetOfPeers(ps, 2*sizeofSlice)
	if len(out) != len(ps) {
		t.Fail()
	}
}
