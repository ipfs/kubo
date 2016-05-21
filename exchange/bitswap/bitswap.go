// package bitswap implements the IPFS Exchange interface with the BitSwap
// bilateral exchange protocol.
package bitswap

import (
	"errors"
	"math"
	"sync"
	"time"

	process "gx/ipfs/QmQopLATEYMNg7dVqZRNDfeE2S1yKy8zrRh5xnYiuqeZBn/goprocess"
	procctx "gx/ipfs/QmQopLATEYMNg7dVqZRNDfeE2S1yKy8zrRh5xnYiuqeZBn/goprocess/context"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	logging "gx/ipfs/QmaDNZ4QMdBdku1YZWBysufYyoQt1negQGNav6PLYarbY8/go-log"
	peer "gx/ipfs/QmbyvM8zRFDkbFdYyt1MnevUMJ62SiSGbfDFZ3Z8nkrzr4/go-libp2p-peer"

	blocks "github.com/ipfs/go-ipfs/blocks"
	blockstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	key "github.com/ipfs/go-ipfs/blocks/key"
	exchange "github.com/ipfs/go-ipfs/exchange"
	decision "github.com/ipfs/go-ipfs/exchange/bitswap/decision"
	bsmsg "github.com/ipfs/go-ipfs/exchange/bitswap/message"
	bsnet "github.com/ipfs/go-ipfs/exchange/bitswap/network"
	notifications "github.com/ipfs/go-ipfs/exchange/bitswap/notifications"
	wantlist "github.com/ipfs/go-ipfs/exchange/bitswap/wantlist"
	flags "github.com/ipfs/go-ipfs/flags"
	"github.com/ipfs/go-ipfs/thirdparty/delay"
	loggables "github.com/ipfs/go-ipfs/thirdparty/loggables"
)

var log = logging.Logger("bitswap")

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
)

var (
	HasBlockBufferSize    = 256
	provideKeysBufferSize = 2048
	provideWorkerMax      = 512
)

func init() {
	if flags.LowMemMode {
		HasBlockBufferSize = 64
		provideKeysBufferSize = 512
		provideWorkerMax = 16
	}
}

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

	bs := &Bitswap{
		self:          p,
		blockstore:    bstore,
		notifications: notif,
		engine:        decision.NewEngine(ctx, bstore), // TODO close the engine with Close() method
		network:       network,
		findKeys:      make(chan *wantlist.Entry, sizeBatchRequestChan),
		process:       px,
		newBlocks:     make(chan blocks.Block, HasBlockBufferSize),
		provideKeys:   make(chan key.Key, provideKeysBufferSize),
		wm:            NewWantManager(ctx, network),
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

	// send keys to a worker to find and connect to providers for them
	findKeys chan *wantlist.Entry

	engine *decision.Engine

	process process.Process

	newBlocks chan blocks.Block

	provideKeys chan key.Key

	counterLk      sync.Mutex
	blocksRecvd    int
	dupBlocksRecvd int
	dupDataRecvd   uint64
}

type blockRequest struct {
	key key.Key
	ctx context.Context
}

// GetBlock attempts to retrieve a particular block from peers within the
// deadline enforced by the context.
func (bs *Bitswap) GetBlock(parent context.Context, k key.Key) (blocks.Block, error) {
	if k == "" {
		return nil, blockstore.ErrNotFound
	}

	// Any async work initiated by this function must end when this function
	// returns. To ensure this, derive a new context. Note that it is okay to
	// listen on parent in this scope, but NOT okay to pass |parent| to
	// functions called by this one. Otherwise those functions won't return
	// when this context's cancel func is executed. This is difficult to
	// enforce. May this comment keep you safe.

	ctx, cancelFunc := context.WithCancel(parent)

	ctx = logging.ContextWithLoggable(ctx, loggables.Uuid("GetBlockRequest"))
	log.Event(ctx, "Bitswap.GetBlockRequest.Start", &k)
	defer log.Event(ctx, "Bitswap.GetBlockRequest.End", &k)

	defer func() {
		cancelFunc()
	}()

	promise, err := bs.GetBlocks(ctx, []key.Key{k})
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

func (bs *Bitswap) WantlistForPeer(p peer.ID) []key.Key {
	var out []key.Key
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
func (bs *Bitswap) GetBlocks(ctx context.Context, keys []key.Key) (<-chan blocks.Block, error) {
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
		log.Event(ctx, "Bitswap.GetBlockRequest.Start", &k)
	}

	bs.wm.WantBlocks(ctx, keys)

	// NB: Optimization. Assumes that providers of key[0] are likely to
	// be able to provide for all keys. This currently holds true in most
	// every situation. Later, this assumption may not hold as true.
	req := &wantlist.Entry{
		Key: keys[0],
		Ctx: ctx,
	}
	select {
	case bs.findKeys <- req:
		return promise, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// CancelWant removes a given key from the wantlist
func (bs *Bitswap) CancelWants(ks []key.Key) {
	bs.wm.CancelWants(ks)
}

// HasBlock announces the existance of a block to this bitswap service. The
// service will potentially notify its peers.
func (bs *Bitswap) HasBlock(blk blocks.Block) error {
	select {
	case <-bs.process.Closing():
		return errors.New("bitswap is closed")
	default:
	}

	err := bs.tryPutBlock(blk, 4) // attempt to store block up to four times
	if err != nil {
		log.Errorf("Error writing block to datastore: %s", err)
		return err
	}

	bs.notifications.Publish(blk)

	bs.engine.AddBlock(blk)

	select {
	case bs.newBlocks <- blk:
		// send block off to be reprovided
	case <-bs.process.Closing():
		return bs.process.Close()
	}
	return nil
}

func (bs *Bitswap) tryPutBlock(blk blocks.Block, attempts int) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = bs.blockstore.Put(blk); err == nil {
			break
		}

		time.Sleep(time.Millisecond * time.Duration(400*(i+1)))
	}
	return err
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
	var keys []key.Key
	for _, block := range iblocks {
		if _, found := bs.wm.wl.Contains(block.Key()); !found {
			log.Infof("received un-asked-for %s from %s", block, p)
			continue
		}
		keys = append(keys, block.Key())
	}
	bs.wm.CancelWants(keys)

	wg := sync.WaitGroup{}
	for _, block := range iblocks {
		wg.Add(1)
		go func(b blocks.Block) {
			defer wg.Done()

			if err := bs.updateReceiveCounters(b); err != nil {
				return // ignore error, is either logged previously, or ErrAlreadyHaveBlock
			}

			k := b.Key()
			log.Event(ctx, "Bitswap.GetBlockRequest.End", &k)

			log.Debugf("got block %s from %s", b, p)
			if err := bs.HasBlock(b); err != nil {
				log.Warningf("ReceiveMessage HasBlock error: %s", err)
			}
		}(block)
	}
	wg.Wait()
}

var ErrAlreadyHaveBlock = errors.New("already have block")

func (bs *Bitswap) updateReceiveCounters(b blocks.Block) error {
	bs.counterLk.Lock()
	defer bs.counterLk.Unlock()
	bs.blocksRecvd++
	has, err := bs.blockstore.Has(b.Key())
	if err != nil {
		log.Infof("blockstore.Has error: %s", err)
		return err
	}
	if err == nil && has {
		bs.dupBlocksRecvd++
		bs.dupDataRecvd += uint64(len(b.Data()))
	}

	if has {
		return ErrAlreadyHaveBlock
	}
	return nil
}

// Connected/Disconnected warns bitswap about peer connections
func (bs *Bitswap) PeerConnected(p peer.ID) {
	bs.wm.Connected(p)
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

func (bs *Bitswap) GetWantlist() []key.Key {
	var out []key.Key
	for _, e := range bs.wm.wl.Entries() {
		out = append(out, e.Key)
	}
	return out
}
