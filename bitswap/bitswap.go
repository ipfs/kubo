package bitswap

import (
	"errors"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"

	bsmsg "github.com/jbenet/go-ipfs/bitswap/message"
	bsnet "github.com/jbenet/go-ipfs/bitswap/network"
	notifications "github.com/jbenet/go-ipfs/bitswap/notifications"
	strategy "github.com/jbenet/go-ipfs/bitswap/strategy"
	blocks "github.com/jbenet/go-ipfs/blocks"
	blockstore "github.com/jbenet/go-ipfs/blockstore"
	peer "github.com/jbenet/go-ipfs/peer"
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

// bitswap instances implement the bitswap protocol.
type bitswap struct {
	// peer is the identity of this (local) node.
	peer *peer.Peer

	// sender delivers messages on behalf of the session
	sender bsnet.NetworkAdapter

	// blockstore is the local database
	// NB: ensure threadsafety
	blockstore blockstore.Blockstore

	// routing interface for communication
	routing Directory

	notifications notifications.PubSub

	// strategist listens to network traffic and makes decisions about how to
	// interact with partners.
	// TODO(brian): save the strategist's state to the datastore
	strategist strategy.Strategist

	// haveList is the set of keys we have values for. a map for fast lookups.
	// haveList KeySet -- not needed. all values in datastore?

	haltChan chan struct{}
}

// NewSession initializes a bitswap session.
func NewSession(parent context.Context, s bsnet.NetworkService, p *peer.Peer, d ds.Datastore, directory Directory) Exchange {

	// FIXME(brian): instantiate a concrete Strategist
	receiver := bsnet.Forwarder{}
	bs := &bitswap{
		peer:          p,
		blockstore:    blockstore.NewBlockstore(d),
		routing:       directory,
		sender:        bsnet.NewNetworkAdapter(s, &receiver),
		haltChan:      make(chan struct{}),
		notifications: notifications.New(),
	}
	receiver.Delegate(bs)

	return bs
}

// GetBlock attempts to retrieve a particular block from peers, within timeout.
func (bs *bitswap) Block(k u.Key, timeout time.Duration) (
	*blocks.Block, error) {
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

func (bs *bitswap) getBlock(k u.Key, p *peer.Peer, timeout time.Duration) (*blocks.Block, error) {

	ctx, _ := context.WithTimeout(context.Background(), timeout)
	blockChannel := bs.notifications.Subscribe(ctx, k)

	message := bsmsg.New()
	message.AppendWanted(k)

	// FIXME(brian): register the accountant on the service wrapper to ensure
	// that accounting is _always_ performed when SendMessage and
	// ReceiveMessage are called
	bs.sender.SendMessage(ctx, p, message)
	bs.strategist.MessageSent(p, message)

	block, ok := <-blockChannel
	if !ok {
		return nil, u.ErrTimeout
	}
	return &block, nil
}

func (bs *bitswap) sendToPeersThatWant(block blocks.Block) {
	for _, p := range bs.strategist.Peers() {
		if bs.strategist.IsWantedByPeer(block.Key(), p) {
			if bs.strategist.ShouldSendToPeer(block.Key(), p) {
				go bs.send(p, block)
			}
		}
	}
}

// HasBlock announces the existance of a block to bitswap, potentially sending
// it to peers (Partners) whose WantLists include it.
func (bs *bitswap) HasBlock(blk blocks.Block) error {
	go bs.sendToPeersThatWant(blk)
	return bs.routing.Provide(blk.Key())
}

// TODO(brian): get a return value
func (bs *bitswap) send(p *peer.Peer, b blocks.Block) {
	message := bsmsg.New()
	message.AppendBlock(b)
	// FIXME(brian): pass ctx
	bs.sender.SendMessage(context.Background(), p, message)
	bs.strategist.MessageSent(p, message)
}

func (bs *bitswap) Halt() {
	bs.haltChan <- struct{}{}
}

// TODO(brian): handle errors
func (bs *bitswap) ReceiveMessage(
	ctx context.Context, sender *peer.Peer, incoming bsmsg.BitSwapMessage) (
	*peer.Peer, bsmsg.BitSwapMessage, error) {

	bs.strategist.MessageReceived(sender, incoming)

	if incoming.Blocks() != nil {
		for _, block := range incoming.Blocks() {
			go bs.blockstore.Put(block) // FIXME(brian): err ignored
			go bs.notifications.Publish(block)
		}
	}

	if incoming.Wantlist() != nil {
		for _, key := range incoming.Wantlist() {
			if bs.strategist.ShouldSendToPeer(key, sender) {
				block, errBlockNotFound := bs.blockstore.Get(key)
				if errBlockNotFound != nil {
					// TODO(brian): log/return the error
					continue
				}
				go bs.send(sender, *block)
			}
		}
	}
	return nil, nil, errors.New("TODO implement")
}

func numBytes(b blocks.Block) int {
	return len(b.Data)
}
