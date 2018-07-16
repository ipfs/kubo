package notifications

import (
	"context"
	"sync"

	blocks "gx/ipfs/QmVzK524a2VWLqyvtBeiHKsUAWYgeAk4DBeZoY7vpNPNRx/go-block-format"
	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	pubsub "gx/ipfs/QmdbxjQWogRCHRaxhhGnYdT1oQJzL9GdqSKzCdqWr85AP2/pubsub"
)

const bufferSize = 16

type PubSub interface {
	Publish(block blocks.Block)
	Subscribe(ctx context.Context, keys ...*cid.Cid) <-chan blocks.Block
	Shutdown()
}

func New() PubSub {
	return &impl{
		wrapped: *pubsub.New(bufferSize),
		cancel:  make(chan struct{}),
	}
}

type impl struct {
	wrapped pubsub.PubSub

	// These two fields make up a shutdown "lock".
	// We need them as calling, e.g., `Unsubscribe` after calling `Shutdown`
	// blocks forever and fixing this in pubsub would be rather invasive.
	cancel chan struct{}
	wg     sync.WaitGroup
}

func (ps *impl) Publish(block blocks.Block) {
	ps.wg.Add(1)
	defer ps.wg.Done()

	select {
	case <-ps.cancel:
		// Already shutdown, bail.
		return
	default:
	}

	ps.wrapped.Pub(block, block.Cid().KeyString())
}

// Not safe to call more than once.
func (ps *impl) Shutdown() {
	// Interrupt in-progress subscriptions.
	close(ps.cancel)
	// Wait for them to finish.
	ps.wg.Wait()
	// shutdown the pubsub.
	ps.wrapped.Shutdown()
}

// Subscribe returns a channel of blocks for the given |keys|. |blockChannel|
// is closed if the |ctx| times out or is cancelled, or after sending len(keys)
// blocks.
func (ps *impl) Subscribe(ctx context.Context, keys ...*cid.Cid) <-chan blocks.Block {

	blocksCh := make(chan blocks.Block, len(keys))
	valuesCh := make(chan interface{}, len(keys)) // provide our own channel to control buffer, prevent blocking
	if len(keys) == 0 {
		close(blocksCh)
		return blocksCh
	}

	// prevent shutdown
	ps.wg.Add(1)

	// check if shutdown *after* preventing shutdowns.
	select {
	case <-ps.cancel:
		// abort, allow shutdown to continue.
		ps.wg.Done()
		close(blocksCh)
		return blocksCh
	default:
	}

	ps.wrapped.AddSubOnceEach(valuesCh, toStrings(keys)...)
	go func() {
		defer func() {
			ps.wrapped.Unsub(valuesCh)
			close(blocksCh)

			// Unblock shutdown.
			ps.wg.Done()
		}()

		for {
			select {
			case <-ps.cancel:
				return
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
				case <-ps.cancel:
					return
				case <-ctx.Done():
					return
				case blocksCh <- block: // continue
				}
			}
		}
	}()

	return blocksCh
}

func toStrings(keys []*cid.Cid) []string {
	strs := make([]string, 0, len(keys))
	for _, key := range keys {
		strs = append(strs, key.KeyString())
	}
	return strs
}
