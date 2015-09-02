// package offline implements an object that implements the exchange
// interface but returns nil values to every request.
package offline

import (
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	blocks "github.com/ipfs/go-ipfs/blocks"
	"github.com/ipfs/go-ipfs/blocks/blockstore"
	key "github.com/ipfs/go-ipfs/blocks/key"
	exchange "github.com/ipfs/go-ipfs/exchange"
)

func Exchange(bs blockstore.Blockstore) exchange.Interface {
	return &offlineExchange{bs: bs}
}

// offlineExchange implements the Exchange interface but doesn't return blocks.
// For use in offline mode.
type offlineExchange struct {
	bs blockstore.Blockstore
}

// GetBlock returns nil to signal that a block could not be retrieved for the
// given key.
// NB: This function may return before the timeout expires.
func (e *offlineExchange) GetBlock(_ context.Context, k key.Key) (*blocks.Block, error) {
	return e.bs.Get(k)
}

// HasBlock always returns nil.
func (e *offlineExchange) HasBlock(b *blocks.Block) error {
	return e.bs.Put(b)
}

// Close always returns nil.
func (_ *offlineExchange) Close() error {
	// NB: exchange doesn't own the blockstore's underlying datastore, so it is
	// not responsible for closing it.
	return nil
}

func (e *offlineExchange) GetBlocks(ctx context.Context, ks []key.Key) (<-chan *blocks.Block, error) {
	out := make(chan *blocks.Block, 0)
	go func() {
		defer close(out)
		var misses []key.Key
		for _, k := range ks {
			hit, err := e.bs.Get(k)
			if err != nil {
				misses = append(misses, k)
				// a long line of misses should abort when context is cancelled.
				select {
				// TODO case send misses down channel
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
			select {
			case out <- hit:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}
