package ping

import (
	"testing"
	"time"

	peer "gx/ipfs/QmUBogf4nUefBjmYjn6jfsfPJRkmDGSeMhNj4usRKq69f4/go-libp2p/p2p/peer"
	netutil "gx/ipfs/QmUBogf4nUefBjmYjn6jfsfPJRkmDGSeMhNj4usRKq69f4/go-libp2p/p2p/test/util"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

func TestPing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h1 := netutil.GenHostSwarm(t, ctx)
	h2 := netutil.GenHostSwarm(t, ctx)

	err := h1.Connect(ctx, peer.PeerInfo{
		ID:    h2.ID(),
		Addrs: h2.Addrs(),
	})

	if err != nil {
		t.Fatal(err)
	}

	ps1 := NewPingService(h1)
	ps2 := NewPingService(h2)

	testPing(t, ps1, h2.ID())
	testPing(t, ps2, h1.ID())
}

func testPing(t *testing.T, ps *PingService, p peer.ID) {
	pctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ts, err := ps.Ping(pctx, p)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 5; i++ {
		select {
		case took := <-ts:
			t.Log("ping took: ", took)
		case <-time.After(time.Second * 4):
			t.Fatal("failed to receive ping")
		}
	}

}
