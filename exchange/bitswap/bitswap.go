package bitswap

import (
	"errors"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"

	blocks "github.com/jbenet/go-ipfs/blocks"
	blockstore "github.com/jbenet/go-ipfs/blockstore"
	exchange "github.com/jbenet/go-ipfs/exchange"
	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	notifications "github.com/jbenet/go-ipfs/exchange/bitswap/notifications"
	strategy "github.com/jbenet/go-ipfs/exchange/bitswap/strategy"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// NetMessageSession initializes a BitSwap session that communicates over the
// provided NetMessage service
func NetMessageSession(parent context.Context, s bsnet.NetMessageService, p *peer.Peer, d ds.Datastore, directory bsnet.Routing) exchange.Interface {

	networkAdapter := bsnet.NetMessageAdapter(s, nil)
	bs := &bitswap{
		blockstore:    blockstore.NewBlockstore(d),
		notifications: notifications.New(),
		strategy:      strategy.New(),
		routing:       directory,
		sender:        networkAdapter,
	}
	networkAdapter.SetDelegate(bs)

	return bs
}

// bitswap instances implement the bitswap protocol.
type bitswap struct {

	// sender delivers messages on behalf of the session
	sender bsnet.Adapter

	// blockstore is the local database
	// NB: ensure threadsafety
	blockstore blockstore.Blockstore

	// routing interface for communication
	routing bsnet.Routing

	notifications notifications.PubSub

	// strategy listens to network traffic and makes decisions about how to
	// interact with partners.
	// TODO(brian): save the strategy's state to the datastore
	strategy strategy.Strategy
}

// GetBlock attempts to retrieve a particular block from peers within the
// deadline enforced by the context
//
// TODO ensure only one active request per key
func (bs *bitswap) Block(parent context.Context, k u.Key) (*blocks.Block, error) {

	ctx, cancelFunc := context.WithCancel(parent)
	promise := bs.notifications.Subscribe(ctx, k)

	go func() {
		const maxProviders = 20
		peersToQuery := bs.routing.FindProvidersAsync(ctx, k, maxProviders)
		message := bsmsg.New()
		message.AppendWanted(k)
		for i := range peersToQuery {
			func(p *peer.Peer) {
				response, err := bs.sender.SendRequest(ctx, p, message)
				if err != nil {
					return
				}
				// FIXME ensure accounting is handled correctly when
				// communication fails. May require slightly different API to
				// get better guarantees. May need shared sequence numbers.
				bs.strategy.MessageSent(p, message)

				if response == nil {
					return
				}
				bs.ReceiveMessage(ctx, p, response)
			}(i)
		}
	}()

	select {
	case block := <-promise:
		cancelFunc()
		return &block, nil
	case <-parent.Done():
		return nil, parent.Err()
	}
}

// HasBlock announces the existance of a block to bitswap, potentially sending
// it to peers (Partners) whose WantLists include it.
func (bs *bitswap) HasBlock(ctx context.Context, blk blocks.Block) error {
	bs.sendToPeersThatWant(ctx, blk)
	return bs.routing.Provide(blk.Key())
}

// TODO(brian): handle errors
func (bs *bitswap) ReceiveMessage(
	ctx context.Context, p *peer.Peer, incoming bsmsg.BitSwapMessage) (
	*peer.Peer, bsmsg.BitSwapMessage, error) {
	if p == nil {
		return nil, nil, errors.New("Received nil Peer")
	}
	if incoming == nil {
		return nil, nil, errors.New("Received nil Message")
	}

	bs.strategy.MessageReceived(p, incoming)

	for _, block := range incoming.Blocks() {
		err := bs.blockstore.Put(block) // FIXME(brian): err ignored
		if err != nil {
			return nil, nil, err
		}
		bs.notifications.Publish(block)
		err = bs.HasBlock(ctx, block) // FIXME err ignored
		if err != nil {
			return nil, nil, err
		}
	}

	for _, key := range incoming.Wantlist() {
		if bs.strategy.ShouldSendBlockToPeer(key, p) {
			block, errBlockNotFound := bs.blockstore.Get(key)
			if errBlockNotFound != nil {
				return nil, nil, errBlockNotFound
			}
			message := bsmsg.New()
			message.AppendBlock(*block)
			defer bs.strategy.MessageSent(p, message)
			return p, message, nil
		}
	}
	return nil, nil, nil
}

// send strives to ensure that accounting is always performed when a message is
// sent
func (bs *bitswap) send(ctx context.Context, p *peer.Peer, m bsmsg.BitSwapMessage) {
	bs.sender.SendMessage(ctx, p, m)
	bs.strategy.MessageSent(p, m)
}

func (bs *bitswap) sendToPeersThatWant(ctx context.Context, block blocks.Block) {
	for _, p := range bs.strategy.Peers() {
		if bs.strategy.BlockIsWantedByPeer(block.Key(), p) {
			if bs.strategy.ShouldSendBlockToPeer(block.Key(), p) {
				message := bsmsg.New()
				message.AppendBlock(block)
				bs.send(ctx, p, message)
			}
		}
	}
}
