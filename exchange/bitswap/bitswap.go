// package bitswap implements the IPFS Exchange interface with the BitSwap
// bilateral exchange protocol.
package bitswap

import (
	"errors"
	"math"
	"sync"
	"time"

	process "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	blocks "github.com/ipfs/go-ipfs/blocks"
	blockstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	exchange "github.com/ipfs/go-ipfs/exchange"
	decision "github.com/ipfs/go-ipfs/exchange/bitswap/decision"
	bsmsg "github.com/ipfs/go-ipfs/exchange/bitswap/message"
	bsnet "github.com/ipfs/go-ipfs/exchange/bitswap/network"
	notifications "github.com/ipfs/go-ipfs/exchange/bitswap/notifications"
	wantlist "github.com/ipfs/go-ipfs/exchange/bitswap/wantlist"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	"github.com/ipfs/go-ipfs/thirdparty/delay"
	eventlog "github.com/ipfs/go-ipfs/thirdparty/eventlog"
	u "github.com/ipfs/go-ipfs/util"
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
	provideTimeout         = time.Second * 15
	sizeBatchRequestChan   = 32
	// kMaxPriority is the max priority as defined by the bitswap protocol
	kMaxPriority = math.MaxInt32

	HasBlockBufferSize = 256
	provideWorkers     = 4
)

var rebroadcastDelay = delay.Fixed(time.Second * 10)

// New initializes a BitSwap instance that communicates over the provided
// BitSwapNetwork. This function registers the returned instance as the network
// delegate.
// Runs until context is cancelled.
func New(parent context.Context, p peer.ID, network bsnet.BitSwapNetwork,
	bstore blockstore.Blockstore, nice bool) exchange.Interface {

	// important to use provided parent context (since it may include important
	// loggable data). It's probably not a good idea to allow bitswap to be
	// coupled to the concerns of the IPFS daemon in this way.
	//
	// FIXME(btc) Now that bitswap manages itself using a process, it probably
	// shouldn't accept a context anymore. Clients should probably use Close()
	// exclusively. We should probably find another way to share logging data
	ctx, cancelFunc := context.WithCancel(parent)

	notif := notifications.New()
	px := process.WithTeardown(func() error {
		notif.Shutdown()
		return nil
	})

	go func() {
		<-px.Closing() // process closes first
		cancelFunc()
	}()
	go func() {
		<-ctx.Done() // parent cancelled first
		px.Close()
	}()

	bs := &Bitswap{
		self:          p,
		blockstore:    bstore,
		notifications: notif,
		engine:        decision.NewEngine(ctx, bstore), // TODO close the engine with Close() method
		network:       network,
		batchRequests: make(chan *blockRequest, sizeBatchRequestChan),
		process:       px,
		newBlocks:     make(chan *blocks.Block, HasBlockBufferSize),
		provideKeys:   make(chan u.Key),
		wm:            NewWantManager(network),
	}
	go bs.wm.Run(ctx)
	network.SetDelegate(bs)

	// Start up bitswaps async worker routines
	bs.startWorkers(px, ctx)
	return bs
}

// Bitswap instances implement the bitswap protocol.
type Bitswap struct {

	// the ID of the peer to act on behalf of
	self peer.ID

	// network delivers messages on behalf of the session
	network bsnet.BitSwapNetwork

	// the peermanager manages sending messages to peers in a way that
	// wont block bitswap operation
	wm *WantManager

	// blockstore is the local database
	// NB: ensure threadsafety
	blockstore blockstore.Blockstore

	notifications notifications.PubSub

	// Requests for a set of related blocks
	// the assumption is made that the same peer is likely to
	// have more than a single block in the set
	batchRequests chan *blockRequest

	engine *decision.Engine

	process process.Process

	newBlocks chan *blocks.Block

	provideKeys chan u.Key

	blocksRecvd    int
	dupBlocksRecvd int
}

type blockRequest struct {
	keys []u.Key
	ctx  context.Context
}

// GetBlock attempts to retrieve a particular block from peers within the
// deadline enforced by the context.
func (bs *Bitswap) GetBlock(parent context.Context, k u.Key) (*blocks.Block, error) {

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
	case block, ok := <-promise:
		if !ok {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				return nil, errors.New("promise channel was closed")
			}
		}
		return block, nil
	case <-parent.Done():
		return nil, parent.Err()
	}
}

func (bs *Bitswap) WantlistForPeer(p peer.ID) []u.Key {
	var out []u.Key
	for _, e := range bs.engine.WantlistForPeer(p) {
		out = append(out, e.Key)
	}
	return out
}

// GetBlocks returns a channel where the caller may receive blocks that
// correspond to the provided |keys|. Returns an error if BitSwap is unable to
// begin this request within the deadline enforced by the context.
//
// NB: Your request remains open until the context expires. To conserve
// resources, provide a context with a reasonably short deadline (ie. not one
// that lasts throughout the lifetime of the server)
func (bs *Bitswap) GetBlocks(ctx context.Context, keys []u.Key) (<-chan *blocks.Block, error) {
	select {
	case <-bs.process.Closing():
		return nil, errors.New("bitswap is closed")
	default:
	}
	promise := bs.notifications.Subscribe(ctx, keys...)

	req := &blockRequest{
		keys: keys,
		ctx:  ctx,
	}
	select {
	case bs.batchRequests <- req:
		return promise, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// HasBlock announces the existance of a block to this bitswap service. The
// service will potentially notify its peers.
func (bs *Bitswap) HasBlock(ctx context.Context, blk *blocks.Block) error {
	select {
	case <-bs.process.Closing():
		return errors.New("bitswap is closed")
	default:
	}

	if err := bs.blockstore.Put(blk); err != nil {
		return err
	}

	bs.notifications.Publish(blk)
	select {
	case bs.newBlocks <- blk:
		// send block off to be reprovided
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (bs *Bitswap) connectToProviders(ctx context.Context, entries []wantlist.Entry) {

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Get providers for all entries in wantlist (could take a while)
	wg := sync.WaitGroup{}
	for _, e := range entries {
		wg.Add(1)
		go func(k u.Key) {
			defer wg.Done()

			child, cancel := context.WithTimeout(ctx, providerRequestTimeout)
			defer cancel()
			providers := bs.network.FindProvidersAsync(child, k, maxProvidersPerRequest)
			for prov := range providers {
				go func(p peer.ID) {
					bs.network.ConnectTo(ctx, p)
				}(prov)
			}
		}(e.Key)
	}

	wg.Wait() // make sure all our children do finish.
}

func (bs *Bitswap) ReceiveMessage(ctx context.Context, p peer.ID, incoming bsmsg.BitSwapMessage) {
	// This call records changes to wantlists, blocks received,
	// and number of bytes transfered.
	bs.engine.MessageReceived(p, incoming)
	// TODO: this is bad, and could be easily abused.
	// Should only track *useful* messages in ledger

	if len(incoming.Blocks()) == 0 {
		return
	}

	// quickly send out cancels, reduces chances of duplicate block receives
	var keys []u.Key
	for _, block := range incoming.Blocks() {
		keys = append(keys, block.Key())
	}
	bs.wm.CancelWants(keys)

	for _, block := range incoming.Blocks() {
		bs.blocksRecvd++
		if has, err := bs.blockstore.Has(block.Key()); err == nil && has {
			bs.dupBlocksRecvd++
		}
		log.Debugf("got block %s from %s", block, p)

		hasBlockCtx, cancel := context.WithTimeout(ctx, hasBlockTimeout)
		if err := bs.HasBlock(hasBlockCtx, block); err != nil {
			log.Warningf("ReceiveMessage HasBlock error: %s", err)
		}
		cancel()
	}
}

// Connected/Disconnected warns bitswap about peer connections
func (bs *Bitswap) PeerConnected(p peer.ID) {
	// TODO: add to clientWorker??
	bs.wm.Connected(p)
}

// Connected/Disconnected warns bitswap about peer connections
func (bs *Bitswap) PeerDisconnected(p peer.ID) {
	bs.wm.Disconnected(p)
	bs.engine.PeerDisconnected(p)
}

func (bs *Bitswap) ReceiveError(err error) {
	log.Debugf("Bitswap ReceiveError: %s", err)
	// TODO log the network error
	// TODO bubble the network error up to the parent context/error logger
}

func (bs *Bitswap) Close() error {
	return bs.process.Close()
}

func (bs *Bitswap) GetWantlist() []u.Key {
	var out []u.Key
	for _, e := range bs.wm.wl.Entries() {
		out = append(out, e.Key)
	}
	return out
}
