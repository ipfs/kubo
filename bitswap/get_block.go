package bitswap

import (
	context "code.google.com/p/go.net/context"

	blocks "github.com/jbenet/go-ipfs/blocks"
	peer "github.com/jbenet/go-ipfs/peer"
	swarm "github.com/jbenet/go-ipfs/swarm"
	u "github.com/jbenet/go-ipfs/util"

	"errors"
	"time"
)

const (
	MaxProvidersForGetBlock = 20
)

/* GetBlock attempts to retrieve the block given by |k| within the timeout
 * period enforced by |ctx|.
 *
 * Once a result is obtained, sends cancellation signal to remaining async
 * workers.
 */
func (bs *BitSwap) GetBlock(ctx context.Context, k u.Key) (
	*blocks.Block, error) {
	u.DOut("Bitswap GetBlock: '%s'\n", k.Pretty())

	var block *blocks.Block
	var err error
	err = bs.emitBlockData(ctx, k, func(blockData []byte, err error) error {
		if err != nil {
			// TODO(brian): optionally log err
		}
		block, err = blocks.NewBlock(blockData)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return block, nil
}

/* Asynchronously retrieves blockData providers. For each provider, retrieves
 * block data. Results collected in blockDataChan and errChan are emitted to
 * |f|.
 *
 * If |f| returns nil, the fan-out is aborted and this function returns. If |f|
 * returns an error, this function emits blocks until the channels are closed
 * or it encounters the deadline enforced by |ctx|.
 *
 * Return values:
 * - If |ctx| signals Done, returns ctx.Err()
 * - Otherwise, returns the return value of the last invocation of |f|.
 */
// TODO(brian): refactor this function so it depends on a function that
// returns a channel of objects |o| which expose functions g such that o.g()
// returns ([]byte, error)
func (bs *BitSwap) emitBlockData(ctx context.Context, k u.Key, f func([]byte, error) error) error {

	_, cancelFunc := context.WithCancel(ctx)

	blockDataChan := make(chan []byte)
	errChan := make(chan error)

	go func() {
		for p := range bs.routing.FindProvidersAsync(ctx, k, MaxProvidersForGetBlock) {
			go func(provider *peer.Peer) {
				block, err := bs.getBlock(ctx, k, provider)
				if err != nil {
					errChan <- err
				} else {
					blockDataChan <- block
				}
			}(p)
		}
	}()

	var err error
	for {
		select {
		case blkdata := <-blockDataChan:
			err = f(blkdata, nil)
			if err == nil {
				cancelFunc()
				return nil
			}
		case err := <-errChan:
			err = f(nil, err)
			if err == nil {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
	// TODO(brian): need to return the last return value of |f|
}

/* Retrieves data for key |k| from peer |p| within timeout enforced by |ctx|.
 */
func (bs *BitSwap) getBlock(ctx context.Context, k u.Key, p *peer.Peer) ([]byte, error) {
	u.DOut("[%s] getBlock '%s' from [%s]\n", bs.peer.ID.Pretty(), k.Pretty(), p.ID.Pretty())

	deadline, ok := ctx.Deadline()
	if !ok {
		return nil, errors.New("Expected caller to provide a deadline")
	}
	timeout := deadline.Sub(time.Now())

	pmes := new(PBMessage)
	pmes.Wantlist = []string{string(k)}

	resp := bs.listener.Listen(string(k), 1, timeout)
	smes := swarm.NewMessage(p, pmes)
	bs.meschan.Outgoing <- smes

	select {
	case resp_mes := <-resp:
		return resp_mes.Data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type blockDataProvider interface {
	ProvidersAsync(ctx context.Context, k u.Key, max int) chan *peer.Peer
	BlockData(ctx context.Context, k u.Key, p *peer.Peer) ([]byte, error)
}
