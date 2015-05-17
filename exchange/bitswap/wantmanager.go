package bitswap

import (
	"sync"
	"time"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	engine "github.com/ipfs/go-ipfs/exchange/bitswap/decision"
	bsmsg "github.com/ipfs/go-ipfs/exchange/bitswap/message"
	bsnet "github.com/ipfs/go-ipfs/exchange/bitswap/network"
	wantlist "github.com/ipfs/go-ipfs/exchange/bitswap/wantlist"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	u "github.com/ipfs/go-ipfs/util"
)

type WantManager struct {
	receiver bsnet.Receiver

	incoming chan []*bsmsg.Entry

	// notification channel for new peers connecting
	connect chan peer.ID

	// notification channel for peers disconnecting
	disconnect chan peer.ID

	peers map[peer.ID]*msgQueue

	wl *wantlist.Wantlist

	network bsnet.BitSwapNetwork

	ctx context.Context
}

func NewWantManager(ctx context.Context, network bsnet.BitSwapNetwork) *WantManager {
	return &WantManager{
		incoming:   make(chan []*bsmsg.Entry, 10),
		connect:    make(chan peer.ID, 10),
		disconnect: make(chan peer.ID, 10),
		peers:      make(map[peer.ID]*msgQueue),
		wl:         wantlist.New(),
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
	blk u.Key
}

type msgQueue struct {
	p peer.ID

	outlk sync.Mutex
	out   bsmsg.BitSwapMessage

	work chan struct{}
	done chan struct{}
}

func (pm *WantManager) WantBlocks(ks []u.Key) {
	pm.addEntries(ks, false)
}

func (pm *WantManager) CancelWants(ks []u.Key) {
	pm.addEntries(ks, true)
}

func (pm *WantManager) addEntries(ks []u.Key, cancel bool) {
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
	err := pm.network.SendMessage(ctx, env.Peer, msg)
	if err != nil {
		log.Error(err)
	}
}

func (pm *WantManager) startPeerHandler(p peer.ID) *msgQueue {
	_, ok := pm.peers[p]
	if ok {
		// TODO: log an error?
		return nil
	}

	mq := newMsgQueue(p)

	// new peer, we will want to give them our full wantlist
	fullwantlist := bsmsg.New(true)
	for _, e := range pm.wl.Entries() {
		fullwantlist.AddEntry(e.Key, e.Priority)
	}
	mq.out = fullwantlist
	mq.work <- struct{}{}

	pm.peers[p] = mq
	go pm.runQueue(mq)
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

func (pm *WantManager) runQueue(mq *msgQueue) {
	for {
		select {
		case <-mq.work: // there is work to be done

			err := pm.network.ConnectTo(pm.ctx, mq.p)
			if err != nil {
				log.Error(err)
				// TODO: cant connect, what now?
			}

			// grab outgoing message
			mq.outlk.Lock()
			wlm := mq.out
			if wlm == nil || wlm.Empty() {
				mq.outlk.Unlock()
				continue
			}
			mq.out = nil
			mq.outlk.Unlock()

			// send wantlist updates
			err = pm.network.SendMessage(pm.ctx, mq.p, wlm)
			if err != nil {
				log.Error("bitswap send error: ", err)
				// TODO: what do we do if this fails?
			}
		case <-mq.done:
			return
		}
	}
}

func (pm *WantManager) Connected(p peer.ID) {
	pm.connect <- p
}

func (pm *WantManager) Disconnected(p peer.ID) {
	pm.disconnect <- p
}

// TODO: use goprocess here once i trust it
func (pm *WantManager) Run() {
	tock := time.NewTicker(rebroadcastDelay.Get())
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

func newMsgQueue(p peer.ID) *msgQueue {
	mq := new(msgQueue)
	mq.done = make(chan struct{})
	mq.work = make(chan struct{}, 1)
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
