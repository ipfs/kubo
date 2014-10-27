package bitswap

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"

	blocks "github.com/jbenet/go-ipfs/blocks"
	blockstore "github.com/jbenet/go-ipfs/blockstore"
	exchange "github.com/jbenet/go-ipfs/exchange"
	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	notifications "github.com/jbenet/go-ipfs/exchange/bitswap/notifications"
	strategy "github.com/jbenet/go-ipfs/exchange/bitswap/strategy"
	inet "github.com/jbenet/go-ipfs/net"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("bitswap")

// NetMessageSession initializes a BitSwap session that communicates over the
// provided NetMessage service
func NetMessageSession(parent context.Context, p peer.Peer,
	net inet.Network, srv inet.Service, directory bsnet.Routing,
	d ds.ThreadSafeDatastore, nice bool) exchange.Interface {

	networkAdapter := bsnet.NetMessageAdapter(srv, net, nil)
	bs := &bitswap{
		blockstore:    blockstore.NewBlockstore(d),
		notifications: notifications.New(),
		strategy:      strategy.New(nice),
		routing:       directory,
		sender:        networkAdapter,
		wantlist:      u.NewKeySet(),
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

	wantlist u.KeySet
}

// GetBlock attempts to retrieve a particular block from peers within the
// deadline enforced by the context
//
// TODO ensure only one active request per key
func (bs *bitswap) Block(parent context.Context, k u.Key) (*blocks.Block, error) {
	log.Debugf("Get Block %v", k)

	ctx, cancelFunc := context.WithCancel(parent)
	bs.wantlist.Add(k)
	promise := bs.notifications.Subscribe(ctx, k)

	const maxProviders = 20
	peersToQuery := bs.routing.FindProvidersAsync(ctx, k, maxProviders)

	go func() {
		message := bsmsg.New()
		for _, wanted := range bs.wantlist.Keys() {
			message.AddWanted(wanted)
		}
		for peerToQuery := range peersToQuery {
			log.Debugf("bitswap got peersToQuery: %s", peerToQuery)
			go func(p peer.Peer) {

				log.Debugf("bitswap dialing peer: %s", p)
				err := bs.sender.DialPeer(p)
				if err != nil {
					log.Errorf("Error sender.DialPeer(%s)", p)
					return
				}

				response, err := bs.sender.SendRequest(ctx, p, message)
				if err != nil {
					log.Error("Error sender.SendRequest(%s) = %s", p, err)
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
			}(peerToQuery)
		}
	}()

	select {
	case block := <-promise:
		cancelFunc()
		bs.wantlist.Remove(k)
		// TODO remove from wantlist
		return &block, nil
	case <-parent.Done():
		return nil, parent.Err()
	}
}

// HasBlock announces the existance of a block to bitswap, potentially sending
// it to peers (Partners) whose WantLists include it.
func (bs *bitswap) HasBlock(ctx context.Context, blk blocks.Block) error {
	log.Debugf("Has Block %v", blk.Key())
	bs.wantlist.Remove(blk.Key())
	bs.sendToPeersThatWant(ctx, blk)
	return bs.routing.Provide(ctx, blk.Key())
}

// TODO(brian): handle errors
func (bs *bitswap) ReceiveMessage(ctx context.Context, p peer.Peer, incoming bsmsg.BitSwapMessage) (
	peer.Peer, bsmsg.BitSwapMessage) {
	log.Debugf("ReceiveMessage from %v", p.Key())
	log.Debugf("Message wantlist: %v", incoming.Wantlist())

	if p == nil {
		log.Error("Received message from nil peer!")
		// TODO propagate the error upward
		return nil, nil
	}
	if incoming == nil {
		log.Error("Got nil bitswap message!")
		// TODO propagate the error upward
		return nil, nil
	}

	// Record message bytes in ledger
	// TODO: this is bad, and could be easily abused.
	// Should only track *useful* messages in ledger
	bs.strategy.MessageReceived(p, incoming) // FIRST

	for _, block := range incoming.Blocks() {
		// TODO verify blocks?
		if err := bs.blockstore.Put(&block); err != nil {
			continue // FIXME(brian): err ignored
		}
		go bs.notifications.Publish(block)
		go func(block blocks.Block) {
			err := bs.HasBlock(ctx, block) // FIXME err ignored
			if err != nil {
				log.Errorf("HasBlock errored: %s", err)
			}
		}(block)
	}

	message := bsmsg.New()
	for _, wanted := range bs.wantlist.Keys() {
		message.AddWanted(wanted)
	}
	for _, key := range incoming.Wantlist() {
		// TODO: might be better to check if we have the block before checking
		//			if we should send it to someone
		if bs.strategy.ShouldSendBlockToPeer(key, p) {
			if block, errBlockNotFound := bs.blockstore.Get(key); errBlockNotFound != nil {
				continue
			} else {
				message.AppendBlock(*block)
			}
		}
	}
	defer bs.strategy.MessageSent(p, message)

	log.Debug("Returning message.")
	return p, message
}

func (bs *bitswap) ReceiveError(err error) {
	log.Errorf("Bitswap ReceiveError: %s", err)
	// TODO log the network error
	// TODO bubble the network error up to the parent context/error logger
}

// send strives to ensure that accounting is always performed when a message is
// sent
func (bs *bitswap) send(ctx context.Context, p peer.Peer, m bsmsg.BitSwapMessage) {
	bs.sender.SendMessage(ctx, p, m)
	go bs.strategy.MessageSent(p, m)
}

func (bs *bitswap) sendToPeersThatWant(ctx context.Context, block blocks.Block) {
	log.Debugf("Sending %v to peers that want it", block.Key())
	for _, p := range bs.strategy.Peers() {
		if bs.strategy.BlockIsWantedByPeer(block.Key(), p) {
			log.Debugf("%v wants %v", p, block.Key())
			if bs.strategy.ShouldSendBlockToPeer(block.Key(), p) {
				message := bsmsg.New()
				message.AppendBlock(block)
				for _, wanted := range bs.wantlist.Keys() {
					message.AddWanted(wanted)
				}
				go bs.send(ctx, p, message)
			}
		}
	}
}
