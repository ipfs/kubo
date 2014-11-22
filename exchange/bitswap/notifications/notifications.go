package notifications

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	pubsub "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/tuxychandru/pubsub"

	blocks "github.com/jbenet/go-ipfs/blocks"
	u "github.com/jbenet/go-ipfs/util"
)

const bufferSize = 16

type PubSub interface {
	Publish(block *blocks.Block)
	Subscribe(ctx context.Context, keys ...u.Key) <-chan *blocks.Block
	Shutdown()
}

func New() PubSub {
	return &impl{*pubsub.New(bufferSize)}
}

type impl struct {
	wrapped pubsub.PubSub
}

func (ps *impl) Publish(block *blocks.Block) {
	topic := string(block.Key())
	ps.wrapped.Pub(block, topic)
}

func (ps *impl) SubscribeDeprec(ctx context.Context, keys ...u.Key) <-chan *blocks.Block {
	topics := make([]string, 0)
	for _, key := range keys {
		topics = append(topics, string(key))
	}
	subChan := ps.wrapped.SubOnce(topics...)
	blockChannel := make(chan *blocks.Block, 1) // buffered so the sender doesn't wait on receiver
	go func() {
		defer close(blockChannel)
		select {
		case val := <-subChan:
			block, ok := val.(*blocks.Block)
			if ok {
				blockChannel <- block
			}
		case <-ctx.Done():
			ps.wrapped.Unsub(subChan, topics...)
		}
	}()
	return blockChannel
}

func (ps *impl) Shutdown() {
	ps.wrapped.Shutdown()
}

// Subscribe returns a channel of blocks for the given |keys|. |blockChannel|
// is closed if the |ctx| times out or is cancelled, or after sending len(keys)
// blocks.
func (ps *impl) Subscribe(ctx context.Context, keys ...u.Key) <-chan *blocks.Block {
	topics := toStrings(keys)
	blocksCh := make(chan *blocks.Block, len(keys))
	valuesCh := make(chan interface{}, len(keys))
	ps.wrapped.AddSub(valuesCh, topics...)

	go func() {
		defer func() {
			ps.wrapped.Unsub(valuesCh, topics...)
			close(blocksCh)
		}()
		for _, _ = range keys {
			select {
			case <-ctx.Done():
				return
			case val, ok := <-valuesCh:
				if !ok {
					return
				}
				block, ok := val.(*blocks.Block)
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case blocksCh <- block: // continue
				}
			}
		}
	}()

	return blocksCh
}

func toStrings(keys []u.Key) []string {
	strs := make([]string, 0)
	for _, key := range keys {
		strs = append(strs, string(key))
	}
	return strs
}
