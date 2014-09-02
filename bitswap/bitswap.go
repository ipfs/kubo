package bitswap

import (
	context "code.google.com/p/go.net/context"

	"code.google.com/p/goprotobuf/proto"
	blocks "github.com/jbenet/go-ipfs/blocks"
	peer "github.com/jbenet/go-ipfs/peer"
	routing "github.com/jbenet/go-ipfs/routing"
	dht "github.com/jbenet/go-ipfs/routing/dht"
	swarm "github.com/jbenet/go-ipfs/swarm"
	u "github.com/jbenet/go-ipfs/util"

	ds "github.com/jbenet/datastore.go"

	"errors"
	"time"
)

// PartnerWantListMax is the bound for the number of keys we'll store per
// partner. These are usually taken from the top of the Partner's WantList
// advertisements. WantLists are sorted in terms of priority.
const PartnerWantListMax = 10

// KeySet is just a convenient alias for maps of keys, where we only care
// access/lookups.
type KeySet map[u.Key]struct{}

// BitSwap instances implement the bitswap protocol.
type BitSwap struct {
	// peer is the identity of this (local) node.
	peer *peer.Peer

	// net holds the connections to all peers.
	net     swarm.Network
	meschan *swarm.Chan

	// datastore is the local database // Ledgers of known
	datastore ds.Datastore

	// routing interface for communication
	routing *dht.IpfsDHT

	listener *swarm.MessageListener

	// partners is a map of currently active bitswap relationships.
	// The Ledger has the peer.ID, and the peer connection works through net.
	// Ledgers of known relationships (active or inactive) stored in datastore.
	// Changes to the Ledger should be committed to the datastore.
	partners map[u.Key]*Ledger

	// haveList is the set of keys we have values for. a map for fast lookups.
	// haveList KeySet -- not needed. all values in datastore?

	// wantList is the set of keys we want values for. a map for fast lookups.
	wantList KeySet

	strategy StrategyFunc

	haltChan chan struct{}
}

// NewBitSwap creates a new BitSwap instance. It does not check its parameters.
func NewBitSwap(p *peer.Peer, net swarm.Network, d ds.Datastore, r routing.IpfsRouting) *BitSwap {
	bs := &BitSwap{
		peer:      p,
		net:       net,
		datastore: d,
		partners:  LedgerMap{},
		wantList:  KeySet{},
		routing:   r.(*dht.IpfsDHT),
		meschan:   net.GetChannel(swarm.PBWrapper_BITSWAP),
		haltChan:  make(chan struct{}),
		listener:  swarm.NewMessageListener(),
	}

	go bs.handleMessages()
	return bs
}

/* GetBlock attempts to retrieve a particular block from peers, within a
 * timeout enforced by |ctx|.
 *
 * Asynchronously fans out the request to many peers |p|. Once a result is
 * obtained, sends cancellation signal to remaining async workers.
 *
 * TODO(brian): "close(valchan)" will panic if worker sends a value after
 * channel is closed. Therefore, senders should manage the channel. Return this
 * channel from a method which (internally) has a goroutine that terminates and
 * closes once all children have responded to the context's termination signal
 */
func (bs *BitSwap) GetBlock(parentCtx context.Context, k u.Key) (
	*blocks.Block, error) {
	u.DOut("Bitswap GetBlock: '%s'\n", k.Pretty())

	ctx, cancelFunc := context.WithCancel(parentCtx)

	const numProvidersDesired = 20
	provs_ch := bs.routing.FindProvidersAsync(ctx, k, numProvidersDesired)

	valchan := make(chan []byte)

	// TODO: when the data is received, shut down this for loop ASAP
	go func() {
		for p := range provs_ch {
			go func(pr *peer.Peer) {
				blk, err := bs.getBlock(ctx, k, pr)
				if err != nil {
					u.PErr("getBlock returned: %v\n", err)
					return
				}
				select {
				case valchan <- blk:
				default:
				}
			}(p)
		}
	}()

	select {
	case blkdata := <-valchan:
		cancelFunc()
		close(valchan)
		return blocks.NewBlock(blkdata)
	case <-ctx.Done():
		// TODO(brian): differentiate between DeadlineExceeded and Cancelled
		return nil, u.ErrTimeout
	}
}

/* Retrieves data for key |k| from peer |p| within timeout enforced by |ctx|.
 */
func (bs *BitSwap) getBlock(ctx context.Context, k u.Key, p *peer.Peer) ([]byte, error) {
	u.DOut("[%s] getBlock '%s' from [%s]\n", bs.peer.ID.Pretty(), k.Pretty(), p.ID.Pretty())

	deadline, ok := ctx.Deadline()
	if !ok {
		return nil, errors.New("Expected caller to provide a deadline")
	}
	timeout := deadline.Sub(time.Now())

	pmes := new(PBMessage)
	pmes.Wantlist = []string{string(k)}

	resp := bs.listener.Listen(string(k), 1, timeout)
	smes := swarm.NewMessage(p, pmes)
	bs.meschan.Outgoing <- smes

	select {
	case resp_mes := <-resp:
		return resp_mes.Data, nil
	case <-ctx.Done():
		u.PErr("getBlock for '%s' timed out.\n", k.Pretty())
		return nil, u.ErrTimeout
	}
}

// HaveBlock announces the existance of a block to BitSwap, potentially sending
// it to peers (Partners) whose WantLists include it.
func (bs *BitSwap) HaveBlock(blk *blocks.Block) error {
	go func() {
		for _, ledger := range bs.partners {
			if _, ok := ledger.WantList[blk.Key()]; ok {
				//send block to node
				if ledger.ShouldSend() {
					bs.SendBlock(ledger.Partner, blk)
				}
			}
		}
	}()
	return bs.routing.Provide(blk.Key())
}

func (bs *BitSwap) SendBlock(p *peer.Peer, b *blocks.Block) {
	pmes := new(PBMessage)
	pmes.Blocks = [][]byte{b.Data}

	swarm_mes := swarm.NewMessage(p, pmes)
	bs.meschan.Outgoing <- swarm_mes
}

func (bs *BitSwap) handleMessages() {
	for {
		select {
		case mes := <-bs.meschan.Incoming:
			pmes := new(PBMessage)
			err := proto.Unmarshal(mes.Data, pmes)
			if err != nil {
				u.PErr("%v\n", err)
				continue
			}
			if pmes.Blocks != nil {
				for _, blkData := range pmes.Blocks {
					blk, err := blocks.NewBlock(blkData)
					if err != nil {
						u.PErr("%v\n", err)
						continue
					}
					go bs.blockReceive(mes.Peer, blk)
				}
			}

			if pmes.Wantlist != nil {
				for _, want := range pmes.Wantlist {
					go bs.peerWantsBlock(mes.Peer, want)
				}
			}
		case <-bs.haltChan:
			return
		}
	}
}

// peerWantsBlock will check if we have the block in question,
// and then if we do, check the ledger for whether or not we should send it.
func (bs *BitSwap) peerWantsBlock(p *peer.Peer, want string) {
	u.DOut("peer [%s] wants block [%s]\n", p.ID.Pretty(), u.Key(want).Pretty())
	ledg := bs.GetLedger(p)

	dsk := ds.NewKey(want)
	blk_i, err := bs.datastore.Get(dsk)
	if err != nil {
		if err == ds.ErrNotFound {
			// TODO: this needs to be different. We need timeouts.
			ledg.WantList[u.Key(want)] = struct{}{}
		}
		u.PErr("datastore get error: %v\n", err)
		return
	}

	blk, ok := blk_i.([]byte)
	if !ok {
		u.PErr("data conversion error.\n")
		return
	}

	if ledg.ShouldSend() {
		u.DOut("Sending block to peer.\n")
		bblk, err := blocks.NewBlock(blk)
		if err != nil {
			u.PErr("newBlock error: %v\n", err)
			return
		}
		bs.SendBlock(p, bblk)
		ledg.SentBytes(len(blk))
	}
}

func (bs *BitSwap) blockReceive(p *peer.Peer, blk *blocks.Block) {
	u.DOut("blockReceive: %s\n", blk.Key().Pretty())
	err := bs.datastore.Put(ds.NewKey(string(blk.Key())), blk.Data)
	if err != nil {
		u.PErr("blockReceive error: %v\n", err)
		return
	}

	mes := &swarm.Message{
		Peer: p,
		Data: blk.Data,
	}
	bs.listener.Respond(string(blk.Key()), mes)

	ledger := bs.GetLedger(p)
	ledger.ReceivedBytes(len(blk.Data))
}

func (bs *BitSwap) GetLedger(p *peer.Peer) *Ledger {
	l, ok := bs.partners[p.Key()]
	if ok {
		return l
	}

	l = new(Ledger)
	l.Strategy = StandardStrategy
	l.Partner = p
	bs.partners[p.Key()] = l
	return l
}

func (bs *BitSwap) SendWantList(wl KeySet) error {
	pmes := new(PBMessage)
	for k, _ := range wl {
		pmes.Wantlist = append(pmes.Wantlist, string(k))
	}

	// Lets just ping everybody all at once
	for _, ledger := range bs.partners {
		bs.meschan.Outgoing <- swarm.NewMessage(ledger.Partner, pmes)
	}

	return nil
}

func (bs *BitSwap) Halt() {
	bs.haltChan <- struct{}{}
}

func (bs *BitSwap) SetStrategy(sf StrategyFunc) {
	bs.strategy = sf
	for _, ledg := range bs.partners {
		ledg.Strategy = sf
	}
}
