package decision

import (
	"fmt"
	"math"
	"testing"

	"github.com/ipfs/go-ipfs/exchange/bitswap/wantlist"
	u "gx/ipfs/QmPsAfmDBnZN3kZGSuNwvCNDZiHneERSKmRcFyG3UkvcT3/go-ipfs-util"
	"gx/ipfs/QmWNY7dV54ZDYmTA1ykVdwNCqC11mpU4zSUp6XDpLTH9eG/go-libp2p-peer"
	"gx/ipfs/QmeDA8gNhvRTsbrjEieay5wezupJDiky8xvCzDABbsGzmp/go-testutil"
	cid "gx/ipfs/QmeSrf6pzut73u6zLQkRFQ3ygt3k6XFT2kjdYP8Tnkwwyg/go-cid"
)

// FWIW: At the time of this commit, including a timestamp in task increases
// time cost of Push by 3%.
func BenchmarkTaskQueuePush(b *testing.B) {
	q := newPRQ()
	peers := []peer.ID{
		testutil.RandPeerIDFatal(b),
		testutil.RandPeerIDFatal(b),
		testutil.RandPeerIDFatal(b),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := cid.NewCidV0(u.Hash([]byte(fmt.Sprint(i))))

		q.Push(&wantlist.Entry{Cid: c, Priority: math.MaxInt32}, peers[i%len(peers)])
	}
}
