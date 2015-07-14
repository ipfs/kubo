package bitswap

import (
	"sync"
	"time"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	key "github.com/ipfs/go-ipfs/blocks/key"
	engine "github.com/ipfs/go-ipfs/exchange/bitswap/decision"
	bsmsg "github.com/ipfs/go-ipfs/exchange/bitswap/message"
	bsnet "github.com/ipfs/go-ipfs/exchange/bitswap/network"
	wantlist "github.com/ipfs/go-ipfs/exchange/bitswap/wantlist"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
)

type WantManager struct {
	// sync channels for Run loop
	incoming   chan []*bsmsg.Entry
	connect    chan peer.ID // notification channel for new peers connecting
	disconnect chan peer.ID // notification channel for peers disconnecting

	// synchronized by Run loop, only touch inside there
	peers map[peer.ID]*msgQueue
	wl    *wantlist.ThreadSafe

	network bsnet.BitSwapNetwork
	ctx     context.Context
}

func NewWantManager(ctx context.Context, network bsnet.BitSwapNetwork) *WantManager {
	return &WantManager{
		incoming:   make(chan []*bsmsg.Entry, 10),
		connect:    make(chan peer.ID, 10),
		disconnect: make(chan peer.ID, 10),
		peers:      make(map[peer.ID]*msgQueue),
		wl:         wantlist.NewThreadSafe(),
		network:    network,
		ctx:        ctx,
	}
}

type msgPair struct {
	to  peer.ID
	msg bsmsg.BitSwapMessage
}

type cancellation struct {
	who peer.ID
	blk key.Key
}

type msgQueue struct {
	p peer.ID

	outlk   sync.Mutex
	out     bsmsg.BitSwapMessage
	network bsnet.BitSwapNetwork

	work chan struct{}
	done chan struct{}
}

func (pm *WantManager) WantBlocks(ks []key.Key) {
	log.Infof("want blocks: %s", ks)
	pm.addEntries(ks, false)
}

func (pm *WantManager) CancelWants(ks []key.Key) {
	pm.addEntries(ks, true)
}

func (pm *WantManager) addEntries(ks []key.Key, cancel bool) {
	var entries []*bsmsg.Entry
	for i, k := range ks {
		entries = append(entries, &bsmsg.Entry{
			Cancel: cancel,
			Entry: wantlist.Entry{
				Key:      k,
				Priority: kMaxPriority - i,
			},
		})
	}
	select {
	case pm.incoming <- entries:
	case <-pm.ctx.Done():
	}
}

func (pm *WantManager) SendBlock(ctx context.Context, env *engine.Envelope) {
	// Blocks need to be sent synchronously to maintain proper backpressure
	// throughout the network stack
	defer env.Sent()

	msg := bsmsg.New(false)
	msg.AddBlock(env.Block)
	log.Infof("Sending block %s to %s", env.Peer, env.Block)
	err := pm.network.SendMessage(ctx, env.Peer, msg)
	if err != nil {
		log.Infof("sendblock error: %s", err)
	}
}

func (pm *WantManager) startPeerHandler(p peer.ID) *msgQueue {
	_, ok := pm.peers[p]
	if ok {
		// TODO: log an error?
		return nil
	}

	mq := pm.newMsgQueue(p)

	// new peer, we will want to give them our full wantlist
	fullwantlist := bsmsg.New(true)
	for _, e := range pm.wl.Entries() {
		fullwantlist.AddEntry(e.Key, e.Priority)
	}
	mq.out = fullwantlist
	mq.work <- struct{}{}

	pm.peers[p] = mq
	go mq.runQueue(pm.ctx)
	return mq
}

func (pm *WantManager) stopPeerHandler(p peer.ID) {
	pq, ok := pm.peers[p]
	if !ok {
		// TODO: log error?
		return
	}

	close(pq.done)
	delete(pm.peers, p)
}

func (mq *msgQueue) runQueue(ctx context.Context) {
	for {
		select {
		case <-mq.work: // there is work to be done
			mq.doWork(ctx)
		case <-mq.done:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (mq *msgQueue) doWork(ctx context.Context) {
	// allow ten minutes for connections
	// this includes looking them up in the dht
	// dialing them, and handshaking
	conctx, cancel := context.WithTimeout(ctx, time.Minute*10)
	defer cancel()

	err := mq.network.ConnectTo(conctx, mq.p)
	if err != nil {
		log.Infof("cant connect to peer %s: %s", mq.p, err)
		// TODO: cant connect, what now?
		return
	}

	// grab outgoing message
	mq.outlk.Lock()
	wlm := mq.out
	if wlm == nil || wlm.Empty() {
		mq.outlk.Unlock()
		return
	}
	mq.out = nil
	mq.outlk.Unlock()

	sendctx, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()

	// send wantlist updates
	err = mq.network.SendMessage(sendctx, mq.p, wlm)
	if err != nil {
		log.Infof("bitswap send error: %s", err)
		// TODO: what do we do if this fails?
		return
	}
}

func (pm *WantManager) Connected(p peer.ID) {
	select {
	case pm.connect <- p:
	case <-pm.ctx.Done():
	}
}

func (pm *WantManager) Disconnected(p peer.ID) {
	select {
	case pm.disconnect <- p:
	case <-pm.ctx.Done():
	}
}

// TODO: use goprocess here once i trust it
func (pm *WantManager) Run() {
	tock := time.NewTicker(rebroadcastDelay.Get())
	defer tock.Stop()
	for {
		select {
		case entries := <-pm.incoming:

			// add changes to our wantlist
			for _, e := range entries {
				if e.Cancel {
					pm.wl.Remove(e.Key)
				} else {
					pm.wl.Add(e.Key, e.Priority)
				}
			}

			// broadcast those wantlist changes
			for _, p := range pm.peers {
				p.addMessage(entries)
			}

		case <-tock.C:
			// resend entire wantlist every so often (REALLY SHOULDNT BE NECESSARY)
			var es []*bsmsg.Entry
			for _, e := range pm.wl.Entries() {
				es = append(es, &bsmsg.Entry{Entry: e})
			}
			for _, p := range pm.peers {
				p.outlk.Lock()
				p.out = bsmsg.New(true)
				p.outlk.Unlock()

				p.addMessage(es)
			}
		case p := <-pm.connect:
			pm.startPeerHandler(p)
		case p := <-pm.disconnect:
			pm.stopPeerHandler(p)
		case <-pm.ctx.Done():
			return
		}
	}
}

func (wm *WantManager) newMsgQueue(p peer.ID) *msgQueue {
	mq := new(msgQueue)
	mq.done = make(chan struct{})
	mq.work = make(chan struct{}, 1)
	mq.network = wm.network
	mq.p = p

	return mq
}

func (mq *msgQueue) addMessage(entries []*bsmsg.Entry) {
	mq.outlk.Lock()
	defer func() {
		mq.outlk.Unlock()
		select {
		case mq.work <- struct{}{}:
		default:
		}
	}()

	// if we have no message held, or the one we are given is full
	// overwrite the one we are holding
	if mq.out == nil {
		mq.out = bsmsg.New(false)
	}

	// TODO: add a msg.Combine(...) method
	// otherwise, combine the one we are holding with the
	// one passed in
	for _, e := range entries {
		if e.Cancel {
			mq.out.Cancel(e.Key)
		} else {
			mq.out.AddEntry(e.Key, e.Priority)
		}
	}
}
