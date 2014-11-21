package async

import (
	"testing"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/blocks"
)

func TestForwardTwo(t *testing.T) {
	const n = 2
	in := make(chan *blocks.Block, n)
	ctx := context.Background()
	out := ForwardN(ctx, in, n)

	in <- blocks.NewBlock([]byte("one"))
	in <- blocks.NewBlock([]byte("two"))

	_ = <-out // 1
	_ = <-out // 2

	_, ok := <-out // closed
	if !ok {
		return
	}
	t.Fail()
}

func TestCloseInput(t *testing.T) {
	const n = 2
	in := make(chan *blocks.Block, 0)
	ctx := context.Background()
	out := ForwardN(ctx, in, n)

	close(in)
	_, ok := <-out // closed
	if !ok {
		return
	}
	t.Fatal("input channel closed, but output channel not")

}

func TestContextClosedWhenBlockingOnInput(t *testing.T) {
	const n = 1 // but we won't ever send a block
	ctx, cancel := context.WithCancel(context.Background())
	out := ForwardN(ctx, make(chan *blocks.Block), n)

	cancel() // before sending anything
	_, ok := <-out
	if !ok {
		return
	}
	t.Fail()
}
