// package bitswap implements the IPFS Exchange interface with the BitSwap
// bilateral exchange protocol.
package bitswap

import (
	"math"
	"sync"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	inflect "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/briantigerchow/inflect"

	blocks "github.com/jbenet/go-ipfs/blocks"
	blockstore "github.com/jbenet/go-ipfs/blocks/blockstore"
	exchange "github.com/jbenet/go-ipfs/exchange"
	decision "github.com/jbenet/go-ipfs/exchange/bitswap/decision"
	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	notifications "github.com/jbenet/go-ipfs/exchange/bitswap/notifications"
	wantlist "github.com/jbenet/go-ipfs/exchange/bitswap/wantlist"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	"github.com/jbenet/go-ipfs/thirdparty/delay"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
	u "github.com/jbenet/go-ipfs/util"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
	pset "github.com/jbenet/go-ipfs/util/peerset" // TODO move this to peerstore
)

var log = eventlog.Logger("bitswap")

const (
	// maxProvidersPerRequest specifies the maximum number of providers desired
	// from the network. This value is specified because the network streams
	// results.
	// TODO: if a 'non-nice' strategy is implemented, consider increasing this value
	maxProvidersPerRequest = 3
	providerRequestTimeout = time.Second * 10
	hasBlockTimeout        = time.Second * 15
	sizeBatchRequestChan   = 32
	// kMaxPriority is the max priority as defined by the bitswap protocol
	kMaxPriority = math.MaxInt32
)

var (
	rebroadcastDelay = delay.Fixed(time.Second * 10)
)

// New initializes a BitSwap instance that communicates over the provided
// BitSwapNetwork. This function registers the returned instance as the network
// delegate.
// Runs until context is cancelled.
func New(parent context.Context, p peer.ID, network bsnet.BitSwapNetwork,
	bstore blockstore.Blockstore, nice bool) exchange.Interface {

	ctx, cancelFunc := context.WithCancel(parent)

	notif := notifications.New()
	go func() {
		<-ctx.Done()
		cancelFunc()
		notif.Shutdown()
	}()

	bs := &bitswap{
		self:          p,
		blockstore:    bstore,
		cancelFunc:    cancelFunc,
		notifications: notif,
		engine:        decision.NewEngine(ctx, bstore),
		network:       network,
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

	// the ID of the peer to act on behalf of
	self peer.ID

	// network delivers messages on behalf of the session
	network bsnet.BitSwapNetwork

	// blockstore is the local database
	// NB: ensure threadsafety
	blockstore blockstore.Blockstore

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
	defer log.EventBegin(ctx, "GetBlockRequest", &k).Done()

	defer func() {
		cancelFunc()
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
	return bs.network.Provide(ctx, blk.Key())
}

func (bs *bitswap) sendWantlistMsgToPeers(ctx context.Context, m bsmsg.BitSwapMessage, peers <-chan peer.ID) error {
	set := pset.New()
	wg := sync.WaitGroup{}
	for peerToQuery := range peers {
		log.Event(ctx, "PeerToQuery", peerToQuery)

		if !set.TryAdd(peerToQuery) { //Do once per peer
			continue
		}

		wg.Add(1)
		go func(p peer.ID) {
			defer wg.Done()
			if err := bs.send(ctx, p, m); err != nil {
				log.Error(err) // TODO remove if too verbose
			}
		}(peerToQuery)
	}
	wg.Wait()
	return nil
}

func (bs *bitswap) sendWantlistToPeers(ctx context.Context, peers <-chan peer.ID) error {
	message := bsmsg.New()
	message.SetFull(true)
	for _, wanted := range bs.wantlist.Entries() {
		message.AddEntry(wanted.Key, wanted.Priority)
	}
	return bs.sendWantlistMsgToPeers(ctx, message, peers)
}

func (bs *bitswap) sendWantlistToProviders(ctx context.Context, entries []wantlist.Entry) {

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// prepare a channel to hand off to sendWantlistToPeers
	sendToPeers := make(chan peer.ID)

	// Get providers for all entries in wantlist (could take a while)
	wg := sync.WaitGroup{}
	for _, e := range entries {
		wg.Add(1)
		go func(k u.Key) {
			defer wg.Done()

			child, _ := context.WithTimeout(ctx, providerRequestTimeout)
			providers := bs.network.FindProvidersAsync(child, k, maxProvidersPerRequest)
			for prov := range providers {
				sendToPeers <- prov
			}
		}(e.Key)
	}

	go func() {
		wg.Wait() // make sure all our children do finish.
		close(sendToPeers)
	}()

	err := bs.sendWantlistToPeers(ctx, sendToPeers)
	if err != nil {
		log.Errorf("sendWantlistToPeers error: %s", err)
	}
}

func (bs *bitswap) taskWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case nextEnvelope := <-bs.engine.Outbox():
			select {
			case <-ctx.Done():
				return
			case envelope, ok := <-nextEnvelope:
				if !ok {
					continue
				}
				bs.send(ctx, envelope.Peer, envelope.Message)
			}
		}
	}
}

// TODO ensure only one active request per key
func (bs *bitswap) clientWorker(parent context.Context) {

	ctx, cancel := context.WithCancel(parent)

	broadcastSignal := time.After(rebroadcastDelay.Get())
	defer cancel()

	for {
		select {
		case <-time.Tick(10 * time.Second):
			n := bs.wantlist.Len()
			if n > 0 {
				log.Debug(n, inflect.FromNumber("keys", n), "in bitswap wantlist")
			}
		case <-broadcastSignal: // resend unfulfilled wantlist keys
			entries := bs.wantlist.Entries()
			if len(entries) > 0 {
				bs.sendWantlistToProviders(ctx, entries)
			}
			broadcastSignal = time.After(rebroadcastDelay.Get())
		case keys := <-bs.batchRequests:
			if len(keys) == 0 {
				log.Warning("Received batch request for zero blocks")
				continue
			}
			for i, k := range keys {
				bs.wantlist.Add(k, kMaxPriority-i)
			}
			// NB: Optimization. Assumes that providers of key[0] are likely to
			// be able to provide for all keys. This currently holds true in most
			// every situation. Later, this assumption may not hold as true.
			child, _ := context.WithTimeout(ctx, providerRequestTimeout)
			providers := bs.network.FindProvidersAsync(child, keys[0], maxProvidersPerRequest)
			err := bs.sendWantlistToPeers(ctx, providers)
			if err != nil {
				log.Errorf("error sending wantlist: %s", err)
			}
		case <-parent.Done():
			return
		}
	}
}

// TODO(brian): handle errors
func (bs *bitswap) ReceiveMessage(ctx context.Context, p peer.ID, incoming bsmsg.BitSwapMessage) (
	peer.ID, bsmsg.BitSwapMessage) {

	if p == "" {
		log.Error("Received message from nil peer!")
		// TODO propagate the error upward
		return "", nil
	}
	if incoming == nil {
		log.Error("Got nil bitswap message!")
		// TODO propagate the error upward
		return "", nil
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
	return "", nil
}

// Connected/Disconnected warns bitswap about peer connections
func (bs *bitswap) PeerConnected(p peer.ID) {
	// TODO: add to clientWorker??
	peers := make(chan peer.ID, 1)
	peers <- p
	close(peers)
	err := bs.sendWantlistToPeers(context.TODO(), peers)
	if err != nil {
		log.Errorf("error sending wantlist: %s", err)
	}
}

// Connected/Disconnected warns bitswap about peer connections
func (bs *bitswap) PeerDisconnected(peer.ID) {
	// TODO: release resources.
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
func (bs *bitswap) send(ctx context.Context, p peer.ID, m bsmsg.BitSwapMessage) error {
	if err := bs.network.SendMessage(ctx, p, m); err != nil {
		return errors.Wrap(err)
	}
	return bs.engine.MessageSent(p, m)
}

func (bs *bitswap) Close() error {
	bs.cancelFunc()
	return nil // to conform to Closer interface
}
