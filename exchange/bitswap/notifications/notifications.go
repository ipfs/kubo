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

func (ps *impl) Shutdown() {
	ps.wrapped.Shutdown()
}

// Subscribe returns a channel of blocks for the given |keys|. |blockChannel|
// is closed if the |ctx| times out or is cancelled, or after sending len(keys)
// blocks.
func (ps *impl) Subscribe(ctx context.Context, keys ...u.Key) <-chan *blocks.Block {

	blocksCh := make(chan *blocks.Block, len(keys))
	valuesCh := make(chan interface{}, len(keys))
	ps.wrapped.AddSub(valuesCh, toStrings(keys)...)

	go func() {
		defer func() {
			ps.wrapped.Unsub(valuesCh, toStrings(keys)...)
			close(blocksCh)
		}()
		seen := make(map[u.Key]struct{})
		i := 0 // req'd because it only counts unique block sends
		for {
			if i >= len(keys) {
				return
			}
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
				if _, ok := seen[block.Key()]; ok {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case blocksCh <- block: // continue
					// Unsub alone is insufficient for keeping out duplicates.
					// It's a race to unsubscribe before pubsub handles the
					// next Publish call. Therefore, must also check for
					// duplicates manually. Unsub is a performance
					// consideration to avoid lots of unnecessary channel
					// chatter.
					ps.wrapped.Unsub(valuesCh, string(block.Key()))
					i++
					seen[block.Key()] = struct{}{}
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
