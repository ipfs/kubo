package notifications

import (
	"bytes"
	"testing"
	"time"

	blocks "github.com/ipfs/go-ipfs/blocks"
	blocksutil "github.com/ipfs/go-ipfs/blocks/blocksutil"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	key "gx/ipfs/Qmce4Y4zg3sYr7xKM5UueS67vhNni6EeWgCRnb7MbLJMew/go-key"
)

func TestDuplicates(t *testing.T) {
	b1 := blocks.NewBlock([]byte("1"))
	b2 := blocks.NewBlock([]byte("2"))

	n := New()
	defer n.Shutdown()
	ch := n.Subscribe(context.Background(), b1.Key(), b2.Key())

	n.Publish(b1)
	blockRecvd, ok := <-ch
	if !ok {
		t.Fail()
	}
	assertBlocksEqual(t, b1, blockRecvd)

	n.Publish(b1) // ignored duplicate

	n.Publish(b2)
	blockRecvd, ok = <-ch
	if !ok {
		t.Fail()
	}
	assertBlocksEqual(t, b2, blockRecvd)
}

func TestPublishSubscribe(t *testing.T) {
	blockSent := blocks.NewBlock([]byte("Greetings from The Interval"))

	n := New()
	defer n.Shutdown()
	ch := n.Subscribe(context.Background(), blockSent.Key())

	n.Publish(blockSent)
	blockRecvd, ok := <-ch
	if !ok {
		t.Fail()
	}

	assertBlocksEqual(t, blockRecvd, blockSent)

}

func TestSubscribeMany(t *testing.T) {
	e1 := blocks.NewBlock([]byte("1"))
	e2 := blocks.NewBlock([]byte("2"))

	n := New()
	defer n.Shutdown()
	ch := n.Subscribe(context.Background(), e1.Key(), e2.Key())

	n.Publish(e1)
	r1, ok := <-ch
	if !ok {
		t.Fatal("didn't receive first expected block")
	}
	assertBlocksEqual(t, e1, r1)

	n.Publish(e2)
	r2, ok := <-ch
	if !ok {
		t.Fatal("didn't receive second expected block")
	}
	assertBlocksEqual(t, e2, r2)
}

// TestDuplicateSubscribe tests a scenario where a given block
// would be requested twice at the same time.
func TestDuplicateSubscribe(t *testing.T) {
	e1 := blocks.NewBlock([]byte("1"))

	n := New()
	defer n.Shutdown()
	ch1 := n.Subscribe(context.Background(), e1.Key())
	ch2 := n.Subscribe(context.Background(), e1.Key())

	n.Publish(e1)
	r1, ok := <-ch1
	if !ok {
		t.Fatal("didn't receive first expected block")
	}
	assertBlocksEqual(t, e1, r1)

	r2, ok := <-ch2
	if !ok {
		t.Fatal("didn't receive second expected block")
	}
	assertBlocksEqual(t, e1, r2)
}

func TestSubscribeIsANoopWhenCalledWithNoKeys(t *testing.T) {
	n := New()
	defer n.Shutdown()
	ch := n.Subscribe(context.Background()) // no keys provided
	if _, ok := <-ch; ok {
		t.Fatal("should be closed if no keys provided")
	}
}

func TestCarryOnWhenDeadlineExpires(t *testing.T) {

	impossibleDeadline := time.Nanosecond
	fastExpiringCtx, cancel := context.WithTimeout(context.Background(), impossibleDeadline)
	defer cancel()

	n := New()
	defer n.Shutdown()
	block := blocks.NewBlock([]byte("A Missed Connection"))
	blockChannel := n.Subscribe(fastExpiringCtx, block.Key())

	assertBlockChannelNil(t, blockChannel)
}

func TestDoesNotDeadLockIfContextCancelledBeforePublish(t *testing.T) {

	g := blocksutil.NewBlockGenerator()
	ctx, cancel := context.WithCancel(context.Background())
	n := New()
	defer n.Shutdown()

	t.Log("generate a large number of blocks. exceed default buffer")
	bs := g.Blocks(1000)
	ks := func() []key.Key {
		var keys []key.Key
		for _, b := range bs {
			keys = append(keys, b.Key())
		}
		return keys
	}()

	_ = n.Subscribe(ctx, ks...) // ignore received channel

	t.Log("cancel context before any blocks published")
	cancel()
	for _, b := range bs {
		n.Publish(b)
	}

	t.Log("publishing the large number of blocks to the ignored channel must not deadlock")
}

func assertBlockChannelNil(t *testing.T, blockChannel <-chan blocks.Block) {
	_, ok := <-blockChannel
	if ok {
		t.Fail()
	}
}

func assertBlocksEqual(t *testing.T, a, b blocks.Block) {
	if !bytes.Equal(a.RawData(), b.RawData()) {
		t.Fatal("blocks aren't equal")
	}
	if a.Key() != b.Key() {
		t.Fatal("block keys aren't equal")
	}
}
