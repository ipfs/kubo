package notifications

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	pubsub "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/tuxychandru/pubsub"

	blocks "github.com/jbenet/go-ipfs/blocks"
	u "github.com/jbenet/go-ipfs/util"
)

type PubSub interface {
	Publish(block *blocks.Block)
	Subscribe(ctx context.Context, k u.Key) <-chan *blocks.Block
	Shutdown()
}

func New() PubSub {
	const bufferSize = 16
	return &impl{*pubsub.New(bufferSize)}
}

type impl struct {
	wrapped pubsub.PubSub
}

func (ps *impl) Publish(block *blocks.Block) {
	topic := string(block.Key())
	ps.wrapped.Pub(block, topic)
}

// Subscribe returns a one-time use |blockChannel|. |blockChannel| returns nil
// if the |ctx| times out or is cancelled. Then channel is closed after the
// block given by |k| is sent.
func (ps *impl) Subscribe(ctx context.Context, k u.Key) <-chan *blocks.Block {
	topic := string(k)
	subChan := ps.wrapped.SubOnce(topic)
	blockChannel := make(chan *blocks.Block)
	go func() {
		defer close(blockChannel)
		select {
		case val := <-subChan:
			block, ok := val.(*blocks.Block)
			if ok {
				blockChannel <- block
			}
		case <-ctx.Done():
			ps.wrapped.Unsub(subChan, topic)
		}
	}()
	return blockChannel
}

func (ps *impl) Shutdown() {
	ps.wrapped.Shutdown()
}
