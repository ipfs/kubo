// package bitswap implements the IPFS exchange interface with the BitSwap
// bilateral exchange protocol.
package bitswap

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"

	blocks "github.com/ipfs/go-ipfs/blocks"
	blockstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	exchange "github.com/ipfs/go-ipfs/exchange"
	decision "github.com/ipfs/go-ipfs/exchange/bitswap/decision"
	bsmsg "github.com/ipfs/go-ipfs/exchange/bitswap/message"
	bsnet "github.com/ipfs/go-ipfs/exchange/bitswap/network"
	notifications "github.com/ipfs/go-ipfs/exchange/bitswap/notifications"
	flags "github.com/ipfs/go-ipfs/flags"
	"github.com/ipfs/go-ipfs/thirdparty/delay"

	metrics "gx/ipfs/QmRg1gKTHzc3CZXSKzem8aR4E3TubFhbgXwfVuWnSK5CC5/go-metrics-interface"
	process "gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess"
	procctx "gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess/context"
	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	loggables "gx/ipfs/QmVesPmqbPp7xRGyY96tnBwzDtVV1nqv4SCVxo5zCqKyH8/go-libp2p-loggables"
	cid "gx/ipfs/QmYhQaCYEcaPPjxJX7YcPcVKkQfRy6sJ7B3XmGFk82XYdQ/go-cid"
	peer "gx/ipfs/QmdS9KpbDyPrieswibZhkod1oXqRwZJrUPzxCofAMWpFGq/go-libp2p-peer"
)

var log = logging.Logger("bitswap")

const (
	// maxProvidersPerRequest specifies the maximum number of providers desired
	// from the network. This value is specified because the network streams
	// results.
	// TODO: if a 'non-nice' strategy is implemented, consider increasing this value
	maxProvidersPerRequest = 3
	providerRequestTimeout = time.Second * 10
	provideTimeout         = time.Second * 15
	sizeBatchRequestChan   = 32
	// kMaxPriority is the max priority as defined by the bitswap protocol
	kMaxPriority = math.MaxInt32
)

var (
	HasBlockBufferSize    = 256
	provideKeysBufferSize = 2048
	provideWorkerMax      = 512

	// the 1<<18+15 is to observe old file chunks that are 1<<18 + 14 in size
	metricsBuckets = []float64{1 << 6, 1 << 10, 1 << 14, 1 << 18, 1<<18 + 15, 1 << 22}
)

func init() {
	if flags.LowMemMode {
		HasBlockBufferSize = 64
		provideKeysBufferSize = 512
		provideWorkerMax = 16
	}
}

var rebroadcastDelay = delay.Fixed(time.Minute)

// New initializes a BitSwap instance that communicates over the provided
// BitSwapNetwork. This function registers the returned instance as the network
// delegate.
// Runs until context is cancelled.
func New(parent context.Context, p peer.ID, network bsnet.BitSwapNetwork,
	bstore blockstore.Blockstore, nice bool) exchange.Interface {

	// important to use provided parent context (since it may include important
	// loggable data). It's probably not a good idea to allow bitswap to be
	// coupled to the concerns of the ipfs daemon in this way.
	//
	// FIXME(btc) Now that bitswap manages itself using a process, it probably
	// shouldn't accept a context anymore. Clients should probably use Close()
	// exclusively. We should probably find another way to share logging data
	ctx, cancelFunc := context.WithCancel(parent)
	ctx = metrics.CtxSubScope(ctx, "bitswap")
	dupHist := metrics.NewCtx(ctx, "recv_dup_blocks_bytes", "Summary of duplicate"+
		" data blocks recived").Histogram(metricsBuckets)
	allHist := metrics.NewCtx(ctx, "recv_all_blocks_bytes", "Summary of all"+
		" data blocks recived").Histogram(metricsBuckets)

	notif := notifications.New()
	px := process.WithTeardown(func() error {
		notif.Shutdown()
		return nil
	})

	bs := &Bitswap{
		blockstore:    bstore,
		notifications: notif,
		engine:        decision.NewEngine(ctx, bstore), // TODO close the engine with Close() method
		network:       network,
		findKeys:      make(chan *blockRequest, sizeBatchRequestChan),
		process:       px,
		newBlocks:     make(chan *cid.Cid, HasBlockBufferSize),
		provideKeys:   make(chan *cid.Cid, provideKeysBufferSize),
		wm:            NewWantManager(ctx, network),

		dupMetric: dupHist,
		allMetric: allHist,
	}
	go bs.wm.Run()
	network.SetDelegate(bs)

	// Start up bitswaps async worker routines
	bs.startWorkers(px, ctx)

	// bind the context and process.
	// do it over here to avoid closing before all setup is done.
	go func() {
		<-px.Closing() // process closes first
		cancelFunc()
	}()
	procctx.CloseAfterContext(px, ctx) // parent cancelled first

	return bs
}

// Bitswap instances implement the bitswap protocol.
type Bitswap struct {
	// the peermanager manages sending messages to peers in a way that
	// wont block bitswap operation
	wm *WantManager

	// the engine is the bit of logic that decides who to send which blocks to
	engine *decision.Engine

	// network delivers messages on behalf of the session
	network bsnet.BitSwapNetwork

	// blockstore is the local database
	// NB: ensure threadsafety
	blockstore blockstore.Blockstore

	// notifications engine for receiving new blocks and routing them to the
	// appropriate user requests
	notifications notifications.PubSub

	// findKeys sends keys to a worker to find and connect to providers for them
	findKeys chan *blockRequest
	// newBlocks is a channel for newly added blocks to be provided to the
	// network.  blocks pushed down this channel get buffered and fed to the
	// provideKeys channel later on to avoid too much network activity
	newBlocks chan *cid.Cid
	// provideKeys directly feeds provide workers
	provideKeys chan *cid.Cid

	process process.Process

	// Counters for various statistics
	counterLk      sync.Mutex
	blocksRecvd    int
	dupBlocksRecvd int
	dupDataRecvd   uint64
	blocksSent     int
	dataSent       uint64
	dataRecvd      uint64

	// Metrics interface metrics
	dupMetric metrics.Histogram
	allMetric metrics.Histogram
}

type blockRequest struct {
	Cid *cid.Cid
	Ctx context.Context
}

// GetBlock attempts to retrieve a particular block from peers within the
// deadline enforced by the context.
func (bs *Bitswap) GetBlock(parent context.Context, k *cid.Cid) (blocks.Block, error) {
	if k == nil {
		log.Error("nil cid in GetBlock")
		return nil, blockstore.ErrNotFound
	}

	// Any async work initiated by this function must end when this function
	// returns. To ensure this, derive a new context. Note that it is okay to
	// listen on parent in this scope, but NOT okay to pass |parent| to
	// functions called by this one. Otherwise those functions won't return
	// when this context's cancel func is executed. This is difficult to
	// enforce. May this comment keep you safe.
	ctx, cancelFunc := context.WithCancel(parent)

	// TODO: this request ID should come in from a higher layer so we can track
	// across multiple 'GetBlock' invocations
	ctx = logging.ContextWithLoggable(ctx, loggables.Uuid("GetBlockRequest"))
	log.Event(ctx, "Bitswap.GetBlockRequest.Start", k)
	defer log.Event(ctx, "Bitswap.GetBlockRequest.End", k)
	defer cancelFunc()

	promise, err := bs.GetBlocks(ctx, []*cid.Cid{k})
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

func (bs *Bitswap) WantlistForPeer(p peer.ID) []*cid.Cid {
	var out []*cid.Cid
	for _, e := range bs.engine.WantlistForPeer(p) {
		out = append(out, e.Cid)
	}
	return out
}

func (bs *Bitswap) LedgerForPeer(p peer.ID) *decision.Receipt {
	return bs.engine.LedgerForPeer(p)
}

// GetBlocks returns a channel where the caller may receive blocks that
// correspond to the provided |keys|. Returns an error if BitSwap is unable to
// begin this request within the deadline enforced by the context.
//
// NB: Your request remains open until the context expires. To conserve
// resources, provide a context with a reasonably short deadline (ie. not one
// that lasts throughout the lifetime of the server)
func (bs *Bitswap) GetBlocks(ctx context.Context, keys []*cid.Cid) (<-chan blocks.Block, error) {
	if len(keys) == 0 {
		out := make(chan blocks.Block)
		close(out)
		return out, nil
	}

	select {
	case <-bs.process.Closing():
		return nil, errors.New("bitswap is closed")
	default:
	}
	promise := bs.notifications.Subscribe(ctx, keys...)

	for _, k := range keys {
		log.Event(ctx, "Bitswap.GetBlockRequest.Start", k)
	}

	bs.wm.WantBlocks(ctx, keys)

	// NB: Optimization. Assumes that providers of key[0] are likely to
	// be able to provide for all keys. This currently holds true in most
	// every situation. Later, this assumption may not hold as true.
	req := &blockRequest{
		Cid: keys[0],
		Ctx: ctx,
	}

	remaining := cid.NewSet()
	for _, k := range keys {
		remaining.Add(k)
	}

	out := make(chan blocks.Block)
	go func() {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		defer close(out)
		defer func() {
			// can't just defer this call on its own, arguments are resolved *when* the defer is created
			bs.CancelWants(remaining.Keys())
		}()
		for {
			select {
			case blk, ok := <-promise:
				if !ok {
					return
				}

				remaining.Remove(blk.Cid())
				select {
				case out <- blk:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	select {
	case bs.findKeys <- req:
		return out, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// CancelWant removes a given key from the wantlist
func (bs *Bitswap) CancelWants(cids []*cid.Cid) {
	bs.wm.CancelWants(cids)
}

// HasBlock announces the existance of a block to this bitswap service. The
// service will potentially notify its peers.
func (bs *Bitswap) HasBlock(blk blocks.Block) error {
	select {
	case <-bs.process.Closing():
		return errors.New("bitswap is closed")
	default:
	}

	err := bs.blockstore.Put(blk)
	if err != nil {
		log.Errorf("Error writing block to datastore: %s", err)
		return err
	}

	// NOTE: There exists the possiblity for a race condition here.  If a user
	// creates a node, then adds it to the dagservice while another goroutine
	// is waiting on a GetBlock for that object, they will receive a reference
	// to the same node. We should address this soon, but i'm not going to do
	// it now as it requires more thought and isnt causing immediate problems.
	bs.notifications.Publish(blk)

	bs.engine.AddBlock(blk)

	select {
	case bs.newBlocks <- blk.Cid():
		// send block off to be reprovided
	case <-bs.process.Closing():
		return bs.process.Close()
	}
	return nil
}

func (bs *Bitswap) ReceiveMessage(ctx context.Context, p peer.ID, incoming bsmsg.BitSwapMessage) {
	// This call records changes to wantlists, blocks received,
	// and number of bytes transfered.
	bs.engine.MessageReceived(p, incoming)
	// TODO: this is bad, and could be easily abused.
	// Should only track *useful* messages in ledger

	iblocks := incoming.Blocks()

	if len(iblocks) == 0 {
		return
	}

	// quickly send out cancels, reduces chances of duplicate block receives
	var keys []*cid.Cid
	for _, block := range iblocks {
		if _, found := bs.wm.wl.Contains(block.Cid()); !found {
			log.Infof("received un-asked-for %s from %s", block, p)
			continue
		}
		keys = append(keys, block.Cid())
	}
	bs.wm.CancelWants(keys)

	wg := sync.WaitGroup{}
	for _, block := range iblocks {
		wg.Add(1)
		go func(b blocks.Block) {
			defer wg.Done()

			bs.updateReceiveCounters(b)

			k := b.Cid()
			log.Event(ctx, "Bitswap.GetBlockRequest.End", k)

			log.Debugf("got block %s from %s", b, p)
			if err := bs.HasBlock(b); err != nil {
				log.Warningf("ReceiveMessage HasBlock error: %s", err)
			}
		}(block)
	}
	wg.Wait()
}

var ErrAlreadyHaveBlock = errors.New("already have block")

func (bs *Bitswap) updateReceiveCounters(b blocks.Block) {
	blkLen := len(b.RawData())
	has, err := bs.blockstore.Has(b.Cid())
	if err != nil {
		log.Infof("blockstore.Has error: %s", err)
		return
	}

	bs.allMetric.Observe(float64(blkLen))
	if has {
		bs.dupMetric.Observe(float64(blkLen))
	}

	bs.counterLk.Lock()
	defer bs.counterLk.Unlock()

	bs.blocksRecvd++
	bs.dataRecvd += uint64(len(b.RawData()))
	if has {
		bs.dupBlocksRecvd++
		bs.dupDataRecvd += uint64(blkLen)
	}
}

// Connected/Disconnected warns bitswap about peer connections
func (bs *Bitswap) PeerConnected(p peer.ID) {
	bs.wm.Connected(p)
	bs.engine.PeerConnected(p)
}

// Connected/Disconnected warns bitswap about peer connections
func (bs *Bitswap) PeerDisconnected(p peer.ID) {
	bs.wm.Disconnected(p)
	bs.engine.PeerDisconnected(p)
}

func (bs *Bitswap) ReceiveError(err error) {
	log.Infof("Bitswap ReceiveError: %s", err)
	// TODO log the network error
	// TODO bubble the network error up to the parent context/error logger
}

func (bs *Bitswap) Close() error {
	return bs.process.Close()
}

func (bs *Bitswap) GetWantlist() []*cid.Cid {
	var out []*cid.Cid
	for _, e := range bs.wm.wl.Entries() {
		out = append(out, e.Cid)
	}
	return out
}

func (bs *Bitswap) IsOnline() bool {
	return true
}
