package decision

import (
	"math"
	"testing"

	"github.com/ipfs/go-ipfs/exchange/bitswap/wantlist"
	"github.com/ipfs/go-ipfs/thirdparty/testutil"
	key "gx/ipfs/QmYEoKZXHoAToWfhGF3vryhMn3WWhE1o2MasQ8uzY5iDi9/go-key"
	"gx/ipfs/QmfMmLGoKzCHDN7cGgk64PJr4iipzidDRME8HABSJqvmhC/go-libp2p-peer"
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
		q.Push(&wantlist.Entry{Key: key.Key(i), Priority: math.MaxInt32}, peers[i%len(peers)])
	}
}
