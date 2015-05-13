package bitswap

import (
	"sync"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	engine "github.com/ipfs/go-ipfs/exchange/bitswap/decision"
	bsmsg "github.com/ipfs/go-ipfs/exchange/bitswap/message"
	bsnet "github.com/ipfs/go-ipfs/exchange/bitswap/network"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	u "github.com/ipfs/go-ipfs/util"
)

type PeerManager struct {
	receiver bsnet.Receiver

	incoming   chan *msgPair
	connect    chan peer.ID
	disconnect chan peer.ID

	peers map[peer.ID]*msgQueue

	network bsnet.BitSwapNetwork
}

func NewPeerManager(network bsnet.BitSwapNetwork) *PeerManager {
	return &PeerManager{
		incoming:   make(chan *msgPair, 10),
		connect:    make(chan peer.ID, 10),
		disconnect: make(chan peer.ID, 10),
		peers:      make(map[peer.ID]*msgQueue),
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

func (pm *PeerManager) SendBlock(env *engine.Envelope) {
	// Blocks need to be sent synchronously to maintain proper backpressure
	// throughout the network stack
	defer env.Sent()

	msg := bsmsg.New()
	msg.AddBlock(env.Block)
	err := pm.network.SendMessage(context.TODO(), env.Peer, msg)
	if err != nil {
		log.Error(err)
	}
}

func (pm *PeerManager) startPeerHandler(p peer.ID) {
	_, ok := pm.peers[p]
	if ok {
		// TODO: log an error?
		return
	}

	mq := new(msgQueue)
	mq.done = make(chan struct{})
	mq.work = make(chan struct{}, 1)
	mq.p = p

	pm.peers[p] = mq
	go pm.runQueue(mq)
}

func (pm *PeerManager) stopPeerHandler(p peer.ID) {
	pq, ok := pm.peers[p]
	if !ok {
		// TODO: log error?
		return
	}

	close(pq.done)
	delete(pm.peers, p)
}

func (pm *PeerManager) runQueue(mq *msgQueue) {
	for {
		select {
		case <-mq.work: // there is work to be done

			// TODO: this might not need to be done every time, figure out
			// a good heuristic
			err := pm.network.ConnectTo(context.TODO(), mq.p)
			if err != nil {
				log.Error(err)
				// TODO: cant connect, what now?
			}

			// grab outgoin message
			mq.outlk.Lock()
			wlm := mq.out
			mq.out = nil
			mq.outlk.Unlock()

			if wlm != nil && !wlm.Empty() {
				// send wantlist updates
				err = pm.network.SendMessage(context.TODO(), mq.p, wlm)
				if err != nil {
					log.Error("bitswap send error: ", err)
					// TODO: what do we do if this fails?
				}
			}
		case <-mq.done:
			return
		}
	}
}

func (pm *PeerManager) Send(to peer.ID, msg bsmsg.BitSwapMessage) {
	if len(msg.Blocks()) > 0 {
		panic("no blocks here!")
	}
	pm.incoming <- &msgPair{to: to, msg: msg}
}

func (pm *PeerManager) Broadcast(msg bsmsg.BitSwapMessage) {
	pm.incoming <- &msgPair{msg: msg}
}

func (pm *PeerManager) Connected(p peer.ID) {
	pm.connect <- p
}

func (pm *PeerManager) Disconnected(p peer.ID) {
	pm.disconnect <- p
}

// TODO: use goprocess here once i trust it
func (pm *PeerManager) Run(ctx context.Context) {
	for {
		select {
		case msgp := <-pm.incoming:

			// Broadcast message to all if recipient not set
			if msgp.to == "" {
				for _, p := range pm.peers {
					p.addMessage(msgp.msg)
				}
				continue
			}

			p, ok := pm.peers[msgp.to]
			if !ok {
				//TODO: decide, drop message? or dial?
				pm.startPeerHandler(msgp.to)
				p = pm.peers[msgp.to]
			}

			p.addMessage(msgp.msg)
		case p := <-pm.connect:
			pm.startPeerHandler(p)
		case p := <-pm.disconnect:
			pm.stopPeerHandler(p)
		case <-ctx.Done():
			return
		}
	}
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
			mq.out.Cancel(e.Key)
		} else {
			mq.out.AddEntry(e.Key, e.Priority)
		}
	}
}
