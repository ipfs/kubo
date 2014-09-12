package bitswap

import (
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"

	notifications "github.com/jbenet/go-ipfs/bitswap/notifications"
	blocks "github.com/jbenet/go-ipfs/blocks"
	swarm "github.com/jbenet/go-ipfs/net/swarm"
	peer "github.com/jbenet/go-ipfs/peer"
	routing "github.com/jbenet/go-ipfs/routing"
	dht "github.com/jbenet/go-ipfs/routing/dht"
	u "github.com/jbenet/go-ipfs/util"
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

	notifications notifications.PubSub

	// partners is a map of currently active bitswap relationships.
	// The Ledger has the peer.ID, and the peer connection works through net.
	// Ledgers of known relationships (active or inactive) stored in datastore.
	// Changes to the Ledger should be committed to the datastore.
	partners LedgerMap

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
		peer:          p,
		net:           net,
		datastore:     d,
		partners:      LedgerMap{},
		wantList:      KeySet{},
		routing:       r.(*dht.IpfsDHT),
		meschan:       net.GetChannel(swarm.PBWrapper_BITSWAP),
		haltChan:      make(chan struct{}),
		notifications: notifications.New(),
	}

	go bs.handleMessages()
	return bs
}

// GetBlock attempts to retrieve a particular block from peers, within timeout.
func (bs *BitSwap) GetBlock(k u.Key, timeout time.Duration) (
	*blocks.Block, error) {
	u.DOut("Bitswap GetBlock: '%s'\n", k.Pretty())
	begin := time.Now()
	tleft := timeout - time.Now().Sub(begin)
	provs_ch := bs.routing.FindProvidersAsync(k, 20, timeout)

	blockChannel := make(chan *blocks.Block)
	after := time.After(tleft)

	// TODO: when the data is received, shut down this for loop ASAP
	go func() {
		for p := range provs_ch {
			go func(pr *peer.Peer) {
				blk, err := bs.getBlock(k, pr, tleft)
				if err != nil {
					u.PErr("getBlock returned: %v\n", err)
					return
				}
				select {
				case blockChannel <- blk:
				default:
				}
			}(p)
		}
	}()

	select {
	case block := <-blockChannel:
		close(blockChannel)
		return block, nil
	case <-after:
		return nil, u.ErrTimeout
	}
}

func (bs *BitSwap) getBlock(k u.Key, p *peer.Peer, timeout time.Duration) (*blocks.Block, error) {
	u.DOut("[%s] getBlock '%s' from [%s]\n", bs.peer.ID.Pretty(), k.Pretty(), p.ID.Pretty())

	ctx, _ := context.WithTimeout(context.Background(), timeout)
	blockChannel := bs.notifications.Subscribe(ctx, k)

	message := newMessage()
	message.AppendWanted(k)
	bs.meschan.Outgoing <- message.ToSwarm(p)

	block, ok := <-blockChannel
	if !ok {
		u.PErr("getBlock for '%s' timed out.\n", k.Pretty())
		return nil, u.ErrTimeout
	}
	return block, nil
}

// HaveBlock announces the existance of a block to BitSwap, potentially sending
// it to peers (Partners) whose WantLists include it.
func (bs *BitSwap) HaveBlock(blk *blocks.Block) error {
	go func() {
		for _, ledger := range bs.partners {
			if ledger.WantListContains(blk.Key()) {
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
	message := newMessage()
	message.AppendBlock(b)
	bs.meschan.Outgoing <- message.ToSwarm(p)
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
			bs.notifications.Shutdown()
			return
		}
	}
}

// peerWantsBlock will check if we have the block in question,
// and then if we do, check the ledger for whether or not we should send it.
func (bs *BitSwap) peerWantsBlock(p *peer.Peer, want string) {
	u.DOut("peer [%s] wants block [%s]\n", p.ID.Pretty(), u.Key(want).Pretty())
	ledger := bs.getLedger(p)

	dsk := ds.NewKey(want)
	blk_i, err := bs.datastore.Get(dsk)
	if err != nil {
		if err == ds.ErrNotFound {
			ledger.Wants(u.Key(want))
		}
		u.PErr("datastore get error: %v\n", err)
		return
	}

	blk, ok := blk_i.([]byte)
	if !ok {
		u.PErr("data conversion error.\n")
		return
	}

	if ledger.ShouldSend() {
		u.DOut("Sending block to peer.\n")
		bblk, err := blocks.NewBlock(blk)
		if err != nil {
			u.PErr("newBlock error: %v\n", err)
			return
		}
		bs.SendBlock(p, bblk)
		ledger.SentBytes(len(blk))
	} else {
		u.DOut("Decided not to send block.")
	}
}

func (bs *BitSwap) blockReceive(p *peer.Peer, blk *blocks.Block) {
	u.DOut("blockReceive: %s\n", blk.Key().Pretty())
	err := bs.datastore.Put(ds.NewKey(string(blk.Key())), blk.Data)
	if err != nil {
		u.PErr("blockReceive error: %v\n", err)
		return
	}

	bs.notifications.Publish(blk)

	ledger := bs.getLedger(p)
	ledger.ReceivedBytes(len(blk.Data))
}

func (bs *BitSwap) getLedger(p *peer.Peer) *Ledger {
	l, ok := bs.partners[p.Key()]
	if ok {
		return l
	}

	l = new(Ledger)
	l.Strategy = bs.strategy
	l.Partner = p
	bs.partners[p.Key()] = l
	return l
}

func (bs *BitSwap) SendWantList(wl KeySet) error {
	message := newMessage()
	for k, _ := range wl {
		message.AppendWanted(k)
	}

	// Lets just ping everybody all at once
	for _, ledger := range bs.partners {
		bs.meschan.Outgoing <- message.ToSwarm(ledger.Partner)
	}

	return nil
}

func (bs *BitSwap) Halt() {
	bs.haltChan <- struct{}{}
}

func (bs *BitSwap) SetStrategy(sf StrategyFunc) {
	bs.strategy = sf
	for _, ledger := range bs.partners {
		ledger.Strategy = sf
	}
}
