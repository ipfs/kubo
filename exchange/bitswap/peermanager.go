package bitswap

import (
	"sync"

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
}

func NewWantManager(network bsnet.BitSwapNetwork) *WantManager {
	return &WantManager{
		incoming:   make(chan []*bsmsg.Entry, 10),
		connect:    make(chan peer.ID, 10),
		disconnect: make(chan peer.ID, 10),
		peers:      make(map[peer.ID]*msgQueue),
		wl:         wantlist.New(),
		network:    network,
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
	log.Error("WANT: ", ks)
	pm.addEntries(ks, false)
}

func (pm *WantManager) CancelWants(ks []u.Key) {
	log.Error("CANCEL: ", ks)
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
	pm.incoming <- entries
}

func (pm *WantManager) SendBlock(ctx context.Context, env *engine.Envelope) {
	// Blocks need to be sent synchronously to maintain proper backpressure
	// throughout the network stack
	defer env.Sent()

	msg := bsmsg.New()
	msg.AddBlock(env.Block)
	msg.SetFull(false)
	err := pm.network.SendMessage(ctx, env.Peer, msg)
	if err != nil {
		log.Error(err)
	}
}

func (pm *WantManager) startPeerHandler(ctx context.Context, p peer.ID) *msgQueue {
	_, ok := pm.peers[p]
	if ok {
		// TODO: log an error?
		return nil
	}

	mq := newMsgQueue(p)

	// new peer, we will want to give them our full wantlist
	fullwantlist := bsmsg.New()
	for _, e := range pm.wl.Entries() {
		fullwantlist.AddEntry(e.Key, e.Priority)
	}
	fullwantlist.SetFull(true)
	mq.out = fullwantlist
	mq.work <- struct{}{}

	pm.peers[p] = mq
	go pm.runQueue(ctx, mq)
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

func (pm *WantManager) runQueue(ctx context.Context, mq *msgQueue) {
	for {
		select {
		case <-mq.work: // there is work to be done

			err := pm.network.ConnectTo(ctx, mq.p)
			if err != nil {
				log.Error(err)
				// TODO: cant connect, what now?
			}

			// grab outgoing message
			mq.outlk.Lock()
			wlm := mq.out
			mq.out = nil
			mq.outlk.Unlock()

			// no message or empty message, continue
			if wlm == nil {
				log.Error("nil wantlist")
				continue
			}
			if wlm.Empty() {
				log.Error("empty wantlist")
				continue
			}

			// send wantlist updates
			err = pm.network.SendMessage(ctx, mq.p, wlm)
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
func (pm *WantManager) Run(ctx context.Context) {
	for {
		select {
		case entries := <-pm.incoming:

			msg := bsmsg.New()
			msg.SetFull(false)
			// add changes to our wantlist
			for _, e := range entries {
				if e.Cancel {
					pm.wl.Remove(e.Key)
					msg.Cancel(e.Key)
				} else {
					pm.wl.Add(e.Key, e.Priority)
					msg.AddEntry(e.Key, e.Priority)
				}
			}

			// broadcast those wantlist changes
			for _, p := range pm.peers {
				p.addMessage(msg)
			}

		case p := <-pm.connect:
			pm.startPeerHandler(ctx, p)
		case p := <-pm.disconnect:
			pm.stopPeerHandler(p)
		case <-ctx.Done():
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

func (mq *msgQueue) addMessage(msg bsmsg.BitSwapMessage) {
	mq.outlk.Lock()
	defer func() {
		mq.outlk.Unlock()
		select {
		case mq.work <- struct{}{}:
		default:
		}
	}()

	if msg.Full() {
		log.Error("GOt FULL MESSAGE")
	}

	// if we have no message held, or the one we are given is full
	// overwrite the one we are holding
	if mq.out == nil || msg.Full() {
		mq.out = msg
		return
	}

	// TODO: add a msg.Combine(...) method
	// otherwise, combine the one we are holding with the
	// one passed in
	for _, e := range msg.Wantlist() {
		if e.Cancel {
			log.Error("add message cancel: ", e.Key, mq.p)
			mq.out.Cancel(e.Key)
		} else {
			log.Error("add message want: ", e.Key, mq.p)
			mq.out.AddEntry(e.Key, e.Priority)
		}
	}
}
