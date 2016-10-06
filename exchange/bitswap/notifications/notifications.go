package notifications

import (
	context "context"
	pubsub "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/briantigerchow/pubsub"
	blocks "github.com/ipfs/go-ipfs/blocks"
	key "gx/ipfs/QmYEoKZXHoAToWfhGF3vryhMn3WWhE1o2MasQ8uzY5iDi9/go-key"
)

const bufferSize = 16

type PubSub interface {
	Publish(block blocks.Block)
	Subscribe(ctx context.Context, keys ...key.Key) <-chan blocks.Block
	Shutdown()
}

func New() PubSub {
	return &impl{*pubsub.New(bufferSize)}
}

type impl struct {
	wrapped pubsub.PubSub
}

func (ps *impl) Publish(block blocks.Block) {
	topic := string(block.Key())
	ps.wrapped.Pub(block, topic)
}

func (ps *impl) Shutdown() {
	ps.wrapped.Shutdown()
}

// Subscribe returns a channel of blocks for the given |keys|. |blockChannel|
// is closed if the |ctx| times out or is cancelled, or after sending len(keys)
// blocks.
func (ps *impl) Subscribe(ctx context.Context, keys ...key.Key) <-chan blocks.Block {

	blocksCh := make(chan blocks.Block, len(keys))
	valuesCh := make(chan interface{}, len(keys)) // provide our own channel to control buffer, prevent blocking
	if len(keys) == 0 {
		close(blocksCh)
		return blocksCh
	}
	ps.wrapped.AddSubOnceEach(valuesCh, toStrings(keys)...)
	go func() {
		defer close(blocksCh)
		defer ps.wrapped.Unsub(valuesCh) // with a len(keys) buffer, this is an optimization
		for {
			select {
			case <-ctx.Done():
				return
			case val, ok := <-valuesCh:
				if !ok {
					return
				}
				block, ok := val.(blocks.Block)
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

func toStrings(keys []key.Key) []string {
	strs := make([]string, 0)
	for _, key := range keys {
		strs = append(strs, string(key))
	}
	return strs
}
