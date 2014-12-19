package decision

import (
	"math"
	"testing"

	"github.com/jbenet/go-ipfs/exchange/bitswap/wantlist"
	"github.com/jbenet/go-ipfs/p2p/peer"
	"github.com/jbenet/go-ipfs/util"
	"github.com/jbenet/go-ipfs/util/testutil"
)

// FWIW: At the time of this commit, including a timestamp in task increases
// time cost of Push by 3%.
func BenchmarkTaskQueuePush(b *testing.B) {
	q := newTaskQueue()
	peers := []peer.ID{
		testutil.RandPeerIDFatal(b),
		testutil.RandPeerIDFatal(b),
		testutil.RandPeerIDFatal(b),
	}
	for i := 0; i < b.N; i++ {
		q.Push(wantlist.Entry{Key: util.Key(i), Priority: math.MaxInt32}, peers[i%len(peers)])
	}
}
