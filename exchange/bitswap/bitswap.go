// package bitswap implements the IPFS Exchange interface with the BitSwap
// bilateral exchange protocol.
package bitswap

import (
	"sync"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	blocks "github.com/jbenet/go-ipfs/blocks"
	blockstore "github.com/jbenet/go-ipfs/blocks/blockstore"
	exchange "github.com/jbenet/go-ipfs/exchange"
	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	notifications "github.com/jbenet/go-ipfs/exchange/bitswap/notifications"
	strategy "github.com/jbenet/go-ipfs/exchange/bitswap/strategy"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	eventlog "github.com/jbenet/go-ipfs/util/eventlog"
)

var log = eventlog.Logger("bitswap")

// Number of providers to request for sending a wantlist to
// TODO: if a 'non-nice' strategy is implemented, consider increasing this value
const maxProvidersPerRequest = 3

const providerRequestTimeout = time.Second * 10
const hasBlockTimeout = time.Second * 15

// New initializes a BitSwap instance that communicates over the
// provided BitSwapNetwork. This function registers the returned instance as
// the network delegate.
// Runs until context is cancelled
func New(parent context.Context, p peer.Peer, network bsnet.BitSwapNetwork, routing bsnet.Routing,
	bstore blockstore.Blockstore, nice bool) exchange.Interface {

	ctx, cancelFunc := context.WithCancel(parent)

	notif := notifications.New()
	go func() {
		<-ctx.Done()
		notif.Shutdown()
	}()

	bs := &bitswap{
		blockstore:    bstore,
		cancelFunc:    cancelFunc,
		notifications: notif,
		strategy:      strategy.New(nice),
		routing:       routing,
		sender:        network,
		wantlist:      u.NewKeySet(),
		batchRequests: make(chan []u.Key, 32),
	}
	network.SetDelegate(bs)
	go bs.loop(ctx)

	return bs
}

// bitswap instances implement the bitswap protocol.
type bitswap struct {

	// sender delivers messages on behalf of the session
	sender bsnet.BitSwapNetwork

	// blockstore is the local database
	// NB: ensure threadsafety
	blockstore blockstore.Blockstore

	// routing interface for communication
	routing bsnet.Routing

	notifications notifications.PubSub

	// Requests for a set of related blocks
	// the assumption is made that the same peer is likely to
	// have more than a single block in the set
	batchRequests chan []u.Key

	// strategy listens to network traffic and makes decisions about how to
	// interact with partners.
	// TODO(brian): save the strategy's state to the datastore
	strategy strategy.Strategy

	wantlist u.KeySet

	// cancelFunc signals cancellation to the bitswap event loop
	cancelFunc func()
}

// GetBlock attempts to retrieve a particular block from peers within the
// deadline enforced by the context.
func (bs *bitswap) GetBlock(parent context.Context, k u.Key) (*blocks.Block, error) {

	// Any async work initiated by this function must end when this function
	// returns. To ensure this, derive a new context. Note that it is okay to
	// listen on parent in this scope, but NOT okay to pass |parent| to
	// functions called by this one. Otherwise those functions won't return
	// when this context's cancel func is executed. This is difficult to
	// enforce. May this comment keep you safe.

	ctx, cancelFunc := context.WithCancel(parent)

	ctx = eventlog.ContextWithLoggable(ctx, eventlog.Uuid("GetBlockRequest"))
	log.Event(ctx, "GetBlockRequestBegin", &k)

	defer func() {
		cancelFunc()
		log.Event(ctx, "GetBlockRequestEnd", &k)
	}()

	promise, err := bs.GetBlocks(ctx, []u.Key{k})
	if err != nil {
		return nil, err
	}

	select {
	case block := <-promise:
		return block, nil
	case <-parent.Done():
		return nil, parent.Err()
	}

}

// GetBlocks returns a channel where the caller may receive blocks that
// correspond to the provided |keys|. Returns an error if BitSwap is unable to
// begin this request within the deadline enforced by the context.
//
// NB: Your request remains open until the context expires. To conserve
// resources, provide a context with a reasonably short deadline (ie. not one
// that lasts throughout the lifetime of the server)
func (bs *bitswap) GetBlocks(ctx context.Context, keys []u.Key) (<-chan *blocks.Block, error) {
	// TODO log the request

	promise := bs.notifications.Subscribe(ctx, keys...)
	select {
	case bs.batchRequests <- keys:
		return promise, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (bs *bitswap) sendWantListTo(ctx context.Context, peers <-chan peer.Peer) error {
	if peers == nil {
		panic("Cant send wantlist to nil peerchan")
	}
	message := bsmsg.New()
	for _, wanted := range bs.wantlist.Keys() {
		message.AddWanted(wanted)
	}
	for peerToQuery := range peers {
		log.Debug("sending query to: %s", peerToQuery)
		log.Event(ctx, "PeerToQuery", peerToQuery)
		go func(p peer.Peer) {

			log.Event(ctx, "DialPeer", p)
			err := bs.sender.DialPeer(ctx, p)
			if err != nil {
				log.Errorf("Error sender.DialPeer(%s): %s", p, err)
				return
			}

			err = bs.sender.SendMessage(ctx, p, message)
			if err != nil {
				log.Errorf("Error sender.SendMessage(%s) = %s", p, err)
				return
			}
			// FIXME ensure accounting is handled correctly when
			// communication fails. May require slightly different API to
			// get better guarantees. May need shared sequence numbers.
			bs.strategy.MessageSent(p, message)
		}(peerToQuery)
	}
	return nil
}

func (bs *bitswap) sendWantlistToProviders(ctx context.Context, ks []u.Key) {
	wg := sync.WaitGroup{}
	for _, k := range ks {
		wg.Add(1)
		go func(k u.Key) {
			child, _ := context.WithTimeout(ctx, providerRequestTimeout)
			providers := bs.routing.FindProvidersAsync(child, k, maxProvidersPerRequest)

			err := bs.sendWantListTo(ctx, providers)
			if err != nil {
				log.Errorf("error sending wantlist: %s", err)
			}
			wg.Done()
		}(k)
	}
	wg.Wait()
}

// TODO ensure only one active request per key
func (bs *bitswap) loop(parent context.Context) {

	ctx, cancel := context.WithCancel(parent)

	broadcastSignal := time.NewTicker(bs.strategy.GetRebroadcastDelay())
	defer func() {
		cancel() // signal to derived async functions
		broadcastSignal.Stop()
	}()

	for {
		select {
		case <-broadcastSignal.C:
			// Resend unfulfilled wantlist keys
			bs.sendWantlistToProviders(ctx, bs.wantlist.Keys())
		case ks := <-bs.batchRequests:
			// TODO: implement batching on len(ks) > X for some X
			//		i.e. if given 20 keys, fetch first five, then next
			//		five, and so on, so we are more likely to be able to
			//		effectively stream the data
			if len(ks) == 0 {
				log.Warning("Received batch request for zero blocks")
				continue
			}
			for _, k := range ks {
				bs.wantlist.Add(k)
			}
			// NB: send want list to providers for the first peer in this list.
			//		the assumption is made that the providers of the first key in
			//		the set are likely to have others as well.
			//		This currently holds true in most every situation, since when
			//		pinning a file, you store and provide all blocks associated with
			//		it. Later, this assumption may not hold as true if we implement
			//		newer bitswap strategies.
			child, _ := context.WithTimeout(ctx, providerRequestTimeout)
			providers := bs.routing.FindProvidersAsync(child, ks[0], maxProvidersPerRequest)

			err := bs.sendWantListTo(ctx, providers)
			if err != nil {
				log.Errorf("error sending wantlist: %s", err)
			}
		case <-parent.Done():
			return
		}
	}
}

// HasBlock announces the existance of a block to this bitswap service. The
// service will potentially notify its peers.
func (bs *bitswap) HasBlock(ctx context.Context, blk *blocks.Block) error {
	if err := bs.blockstore.Put(blk); err != nil {
		return err
	}
	bs.wantlist.Remove(blk.Key())
	bs.notifications.Publish(blk)
	child, _ := context.WithTimeout(ctx, hasBlockTimeout)
	if err := bs.sendToPeersThatWant(child, blk); err != nil {
		return err
	}
	child, _ = context.WithTimeout(ctx, hasBlockTimeout)
	return bs.routing.Provide(child, blk.Key())
}

// TODO(brian): handle errors
func (bs *bitswap) ReceiveMessage(ctx context.Context, p peer.Peer, incoming bsmsg.BitSwapMessage) (
	peer.Peer, bsmsg.BitSwapMessage) {
	log.Debugf("ReceiveMessage from %s", p)

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
	// This call records changes to wantlists, blocks received,
	// and number of bytes transfered.
	bs.strategy.MessageReceived(p, incoming)

	go func() {
		for _, block := range incoming.Blocks() {
			if err := bs.HasBlock(ctx, block); err != nil {
				log.Error(err)
			}
		}
	}()

	for _, key := range incoming.Wantlist() {
		if bs.strategy.ShouldSendBlockToPeer(key, p) {
			if block, errBlockNotFound := bs.blockstore.Get(key); errBlockNotFound != nil {
				continue
			} else {
				// Create a separate message to send this block in
				blkmsg := bsmsg.New()

				// TODO: only send this the first time
				//		no sense in sending our wantlist to the
				//		same peer multiple times
				for _, k := range bs.wantlist.Keys() {
					blkmsg.AddWanted(k)
				}

				blkmsg.AddBlock(block)
				bs.send(ctx, p, blkmsg)
			}
		}
	}

	// TODO: consider changing this function to not return anything
	return nil, nil
}

func (bs *bitswap) ReceiveError(err error) {
	log.Errorf("Bitswap ReceiveError: %s", err)
	// TODO log the network error
	// TODO bubble the network error up to the parent context/error logger
}

// send strives to ensure that accounting is always performed when a message is
// sent
func (bs *bitswap) send(ctx context.Context, p peer.Peer, m bsmsg.BitSwapMessage) error {
	if err := bs.sender.SendMessage(ctx, p, m); err != nil {
		return err
	}
	return bs.strategy.MessageSent(p, m)
}

func (bs *bitswap) sendToPeersThatWant(ctx context.Context, block *blocks.Block) error {
	for _, p := range bs.strategy.Peers() {
		if bs.strategy.BlockIsWantedByPeer(block.Key(), p) {
			if bs.strategy.ShouldSendBlockToPeer(block.Key(), p) {
				message := bsmsg.New()
				message.AddBlock(block)
				for _, wanted := range bs.wantlist.Keys() {
					message.AddWanted(wanted)
				}
				if err := bs.send(ctx, p, message); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (bs *bitswap) Close() error {
	bs.cancelFunc()
	return nil // to conform to Closer interface
}
