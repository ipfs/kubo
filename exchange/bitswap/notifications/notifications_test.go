package notifications

import (
	"bytes"
	"testing"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	blocks "github.com/jbenet/go-ipfs/blocks"
)

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

func TestCarryOnWhenDeadlineExpires(t *testing.T) {

	impossibleDeadline := time.Nanosecond
	fastExpiringCtx, _ := context.WithTimeout(context.Background(), impossibleDeadline)

	n := New()
	defer n.Shutdown()
	block := blocks.NewBlock([]byte("A Missed Connection"))
	blockChannel := n.Subscribe(fastExpiringCtx, block.Key())

	assertBlockChannelNil(t, blockChannel)
}

func assertBlockChannelNil(t *testing.T, blockChannel <-chan *blocks.Block) {
	_, ok := <-blockChannel
	if ok {
		t.Fail()
	}
}

func assertBlocksEqual(t *testing.T, a, b *blocks.Block) {
	if !bytes.Equal(a.Data, b.Data) {
		t.Fail()
	}
	if a.Key() != b.Key() {
		t.Fail()
	}
}
