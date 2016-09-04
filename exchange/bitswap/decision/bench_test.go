package decision

import (
	"math"
	"testing"

	"github.com/ipfs/go-ipfs/exchange/bitswap/wantlist"
	key "github.com/ipfs/go-key"
	"github.com/ipfs/go-libp2p-peer"
	"github.com/libp2p/go-testutil"
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
		q.Push(wantlist.Entry{Key: key.Key(i), Priority: math.MaxInt32}, peers[i%len(peers)])
	}
}
