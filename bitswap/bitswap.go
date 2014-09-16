package bitswap

import (
	"errors"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"

	bsmsg "github.com/jbenet/go-ipfs/bitswap/message"
	bsnet "github.com/jbenet/go-ipfs/bitswap/network"
	notifications "github.com/jbenet/go-ipfs/bitswap/notifications"
	blocks "github.com/jbenet/go-ipfs/blocks"
	blockstore "github.com/jbenet/go-ipfs/blockstore"
	peer "github.com/jbenet/go-ipfs/peer"
	routing "github.com/jbenet/go-ipfs/routing"
	u "github.com/jbenet/go-ipfs/util"
)

// TODO(brian): ensure messages are being received

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

	// sender delivers messages on behalf of the session
	sender bsnet.NetworkAdapter

	// blockstore is the local database
	blockstore blockstore.Blockstore

	// routing interface for communication
	routing routing.IpfsRouting

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

// NewSession initializes a bitswap session.
func NewSession(parent context.Context, s bsnet.NetworkService, p *peer.Peer, d ds.Datastore, r routing.IpfsRouting) *BitSwap {

	receiver := bsnet.Forwarder{}
	bs := &BitSwap{
		peer:          p,
		blockstore:    blockstore.NewBlockstore(d),
		partners:      LedgerMap{},
		wantList:      KeySet{},
		routing:       r,
		sender:        bsnet.NewNetworkAdapter(s, &receiver),
		haltChan:      make(chan struct{}),
		notifications: notifications.New(),
		strategy:      YesManStrategy,
	}
	receiver.Delegate(bs)

	return bs
}

// GetBlock attempts to retrieve a particular block from peers, within timeout.
func (bs *BitSwap) Block(k u.Key, timeout time.Duration) (
	*blocks.Block, error) {
	u.DOut("Bitswap GetBlock: '%s'\n", k.Pretty())
	begin := time.Now()
	tleft := timeout - time.Now().Sub(begin)
	provs_ch := bs.routing.FindProvidersAsync(k, 20, timeout)

	blockChannel := make(chan blocks.Block)
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
				case blockChannel <- *blk:
				default:
				}
			}(p)
		}
	}()

	select {
	case block := <-blockChannel:
		close(blockChannel)
		return &block, nil
	case <-after:
		return nil, u.ErrTimeout
	}
}

func (bs *BitSwap) getBlock(k u.Key, p *peer.Peer, timeout time.Duration) (*blocks.Block, error) {
	u.DOut("[%s] getBlock '%s' from [%s]\n", bs.peer.ID.Pretty(), k.Pretty(), p.ID.Pretty())

	ctx, _ := context.WithTimeout(context.Background(), timeout)
	blockChannel := bs.notifications.Subscribe(ctx, k)

	message := bsmsg.New()
	message.AppendWanted(k)
	bs.sender.SendMessage(ctx, p, message)

	block, ok := <-blockChannel
	if !ok {
		u.PErr("getBlock for '%s' timed out.\n", k.Pretty())
		return nil, u.ErrTimeout
	}
	return &block, nil
}

// HasBlock announces the existance of a block to BitSwap, potentially sending
// it to peers (Partners) whose WantLists include it.
func (bs *BitSwap) HasBlock(blk blocks.Block) error {
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

// TODO(brian): get a return value
func (bs *BitSwap) SendBlock(p *peer.Peer, b blocks.Block) {
	u.DOut("Sending block to peer.\n")
	message := bsmsg.New()
	// TODO(brian): change interface to accept value instead of pointer
	message.AppendBlock(b)
	bs.sender.SendMessage(context.Background(), p, message)
}

// peerWantsBlock will check if we have the block in question,
// and then if we do, check the ledger for whether or not we should send it.
func (bs *BitSwap) peerWantsBlock(p *peer.Peer, wanted u.Key) {
	u.DOut("peer [%s] wants block [%s]\n", p.ID.Pretty(), wanted.Pretty())

	ledger := bs.getLedger(p)

	if !ledger.ShouldSend() {
		return
	}

	block, err := bs.blockstore.Get(wanted)
	if err != nil { // TODO(brian): log/return the error
		ledger.Wants(wanted)
		return
	}
	bs.SendBlock(p, *block)
	ledger.SentBytes(numBytes(*block))
}

// TODO(brian): return error
func (bs *BitSwap) blockReceive(p *peer.Peer, blk blocks.Block) {
	u.DOut("blockReceive: %s\n", blk.Key().Pretty())
	err := bs.blockstore.Put(blk)
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
	message := bsmsg.New()
	for k, _ := range wl {
		message.AppendWanted(k)
	}

	// Lets just ping everybody all at once
	for _, ledger := range bs.partners {
		bs.sender.SendMessage(context.TODO(), ledger.Partner, message)
	}

	return nil
}

func (bs *BitSwap) Halt() {
	bs.haltChan <- struct{}{}
}

func (bs *BitSwap) ReceiveMessage(
	ctx context.Context, sender *peer.Peer, incoming bsmsg.BitSwapMessage) (
	*peer.Peer, bsmsg.BitSwapMessage, error) {
	if incoming.Blocks() != nil {
		for _, block := range incoming.Blocks() {
			go bs.blockReceive(sender, block)
		}
	}

	if incoming.Wantlist() != nil {
		for _, want := range incoming.Wantlist() {
			// TODO(brian): return the block synchronously
			go bs.peerWantsBlock(sender, want)
		}
	}
	return nil, nil, errors.New("TODO implement")
}

func numBytes(b blocks.Block) int {
	return len(b.Data)
}
