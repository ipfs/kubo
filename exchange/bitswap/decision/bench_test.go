package decision

import (
	"fmt"
	"math"
	"testing"

	"github.com/ipfs/go-ipfs/exchange/bitswap/wantlist"
	"github.com/ipfs/go-ipfs/thirdparty/testutil"
	cid "gx/ipfs/QmNw61A6sJoXMeP37mJRtQZdNhj5e3FdjoTN3v4FyE96Gk/go-cid"
	u "gx/ipfs/QmWbjfz3u6HkAdPh34dgPchGbQjob6LXLhAeCGii2TX69n/go-ipfs-util"
	"gx/ipfs/QmdS9KpbDyPrieswibZhkod1oXqRwZJrUPzxCofAMWpFGq/go-libp2p-peer"
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
