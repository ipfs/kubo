// package bitswap implements the IPFS Exchange interface with the BitSwap
// bilateral exchange protocol.
package bitswap

import (
	"math"
	"sync"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	blocks "github.com/jbenet/go-ipfs/blocks"
	blockstore "github.com/jbenet/go-ipfs/blocks/blockstore"
	exchange "github.com/jbenet/go-ipfs/exchange"
	decision "github.com/jbenet/go-ipfs/exchange/bitswap/decision"
	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	notifications "github.com/jbenet/go-ipfs/exchange/bitswap/notifications"
	wantlist "github.com/jbenet/go-ipfs/exchange/bitswap/wantlist"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	eventlog "github.com/jbenet/go-ipfs/util/eventlog"
	pset "github.com/jbenet/go-ipfs/util/peerset"
)

var log = eventlog.Logger("bitswap")

const (
	// Number of providers to request for sending a wantlist to
	// TODO: if a 'non-nice' strategy is implemented, consider increasing this value
	maxProvidersPerRequest = 3
	providerRequestTimeout = time.Second * 10
	hasBlockTimeout        = time.Second * 15
	sizeBatchRequestChan   = 32
	// kMaxPriority is the max priority as defined by the bitswap protocol
	kMaxPriority = math.MaxInt32
)

var (
	rebroadcastDelay = time.Second * 10
)

// New initializes a BitSwap instance that communicates over the provided
// BitSwapNetwork. This function registers the returned instance as the network
// delegate.
// Runs until context is cancelled.
func New(parent context.Context, p peer.Peer, network bsnet.BitSwapNetwork, routing bsnet.Routing,
	bstore blockstore.Blockstore, nice bool) exchange.Interface {

	ctx, cancelFunc := context.WithCancel(parent)

	notif := notifications.New()
	go func() {
		<-ctx.Done()
		cancelFunc()
		notif.Shutdown()
	}()

	bs := &bitswap{
		blockstore:    bstore,
		cancelFunc:    cancelFunc,
		notifications: notif,
		engine:        decision.NewEngine(ctx, bstore),
		routing:       routing,
		sender:        network,
		wantlist:      wantlist.NewThreadSafe(),
		batchRequests: make(chan []u.Key, sizeBatchRequestChan),
	}
	network.SetDelegate(bs)
	go bs.clientWorker(ctx)
	go bs.taskWorker(ctx)

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

	engine *decision.Engine

	wantlist *wantlist.ThreadSafe

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

// HasBlock announces the existance of a block to this bitswap service. The
// service will potentially notify its peers.
func (bs *bitswap) HasBlock(ctx context.Context, blk *blocks.Block) error {
	if err := bs.blockstore.Put(blk); err != nil {
		return err
	}
	bs.wantlist.Remove(blk.Key())
	bs.notifications.Publish(blk)
	return bs.routing.Provide(ctx, blk.Key())
}

func (bs *bitswap) sendWantListTo(ctx context.Context, peers <-chan peer.Peer) error {
	if peers == nil {
		panic("Cant send wantlist to nil peerchan")
	}
	message := bsmsg.New()
	for _, wanted := range bs.wantlist.Entries() {
		message.AddEntry(wanted.Key, wanted.Priority)
	}
	wg := sync.WaitGroup{}
	for peerToQuery := range peers {
		log.Event(ctx, "PeerToQuery", peerToQuery)
		wg.Add(1)
		go func(p peer.Peer) {
			defer wg.Done()

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
			bs.engine.MessageSent(p, message)
		}(peerToQuery)
	}
	wg.Wait()
	return nil
}

func (bs *bitswap) sendWantlistToProviders(ctx context.Context, wantlist *wantlist.ThreadSafe) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	message := bsmsg.New()
	message.SetFull(true)
	for _, e := range bs.wantlist.Entries() {
		message.AddEntry(e.Key, e.Priority)
	}

	ps := pset.New()

	// Get providers for all entries in wantlist (could take a while)
	wg := sync.WaitGroup{}
	for _, e := range wantlist.Entries() {
		wg.Add(1)
		go func(k u.Key) {
			defer wg.Done()
			child, _ := context.WithTimeout(ctx, providerRequestTimeout)
			providers := bs.routing.FindProvidersAsync(child, k, maxProvidersPerRequest)

			for prov := range providers {
				if ps.TryAdd(prov) { //Do once per peer
					bs.send(ctx, prov, message)
				}
			}
		}(e.Key)
	}
	wg.Wait()
}

func (bs *bitswap) taskWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case envelope := <-bs.engine.Outbox():
			bs.send(ctx, envelope.Peer, envelope.Message)
		}
	}
}

// TODO ensure only one active request per key
func (bs *bitswap) clientWorker(parent context.Context) {

	ctx, cancel := context.WithCancel(parent)

	broadcastSignal := time.After(rebroadcastDelay)
	defer cancel()

	for {
		select {
		case <-broadcastSignal:
			// Resend unfulfilled wantlist keys
			bs.sendWantlistToProviders(ctx, bs.wantlist)
			broadcastSignal = time.After(rebroadcastDelay)
		case ks := <-bs.batchRequests:
			if len(ks) == 0 {
				log.Warning("Received batch request for zero blocks")
				continue
			}
			for i, k := range ks {
				bs.wantlist.Add(k, kMaxPriority-i)
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

	// This call records changes to wantlists, blocks received,
	// and number of bytes transfered.
	bs.engine.MessageReceived(p, incoming)
	// TODO: this is bad, and could be easily abused.
	// Should only track *useful* messages in ledger

	for _, block := range incoming.Blocks() {
		hasBlockCtx, _ := context.WithTimeout(ctx, hasBlockTimeout)
		if err := bs.HasBlock(hasBlockCtx, block); err != nil {
			log.Error(err)
		}
	}
	var keys []u.Key
	for _, block := range incoming.Blocks() {
		keys = append(keys, block.Key())
	}
	bs.cancelBlocks(ctx, keys)

	// TODO: consider changing this function to not return anything
	return nil, nil
}

func (bs *bitswap) cancelBlocks(ctx context.Context, bkeys []u.Key) {
	if len(bkeys) < 1 {
		return
	}
	message := bsmsg.New()
	message.SetFull(false)
	for _, k := range bkeys {
		message.Cancel(k)
	}
	for _, p := range bs.engine.Peers() {
		err := bs.send(ctx, p, message)
		if err != nil {
			log.Errorf("Error sending message: %s", err)
		}
	}
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
	return bs.engine.MessageSent(p, m)
}

func (bs *bitswap) Close() error {
	bs.cancelFunc()
	return nil // to conform to Closer interface
}
