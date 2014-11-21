package async

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/blocks"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("async")

// ForwardN forwards up to |num| blocks to the returned channel.
func ForwardN(ctx context.Context, in <-chan *blocks.Block, num int) <-chan *blocks.Block {
	out := make(chan *blocks.Block)
	go func() {
		defer close(out)
		for i := 0; i < num; i++ {
			select {
			case block, ok := <-in:
				if !ok {
					log.Error("Forwarder exiting early!")
					return // otherwise nil value is forwarded to output
				}
				select {
				case out <- block:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}
