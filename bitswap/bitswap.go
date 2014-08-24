package bitswap

import (
	"code.google.com/p/goprotobuf/proto"
	blocks "github.com/jbenet/go-ipfs/blocks"
	peer "github.com/jbenet/go-ipfs/peer"
	routing "github.com/jbenet/go-ipfs/routing"
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

	// datastore is the local database
	// Ledgers of known
	datastore ds.Datastore

	// routing interface for communication
	routing routing.IpfsRouting

	listener *swarm.MesListener

	// partners is a map of currently active bitswap relationships.
	// The Ledger has the peer.ID, and the peer connection works through net.
	// Ledgers of known relationships (active or inactive) stored in datastore.
	// Changes to the Ledger should be committed to the datastore.
	partners map[u.Key]*Ledger

	// haveList is the set of keys we have values for. a map for fast lookups.
	// haveList KeySet -- not needed. all values in datastore?

	// wantList is the set of keys we want values for. a map for fast lookups.
	wantList KeySet

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
		routing:   r,
		meschan:   net.GetChannel(swarm.PBWrapper_BITSWAP),
		haltChan:  make(chan struct{}),
	}

	go bs.handleMessages()
	return bs
}

// GetBlock attempts to retrieve a particular block from peers, within timeout.
func (bs *BitSwap) GetBlock(k u.Key, timeout time.Time) (
	*blocks.Block, error) {
	begin := time.Now()
	_, err := bs.routing.FindProviders(k, timeout)
	if err != nil {
		u.PErr("GetBlock error: %s\n", err)
		return
	}
	tleft := timeout.Sub(time.Now().Sub(begin))
	return nil, errors.New("not implemented")
}

func (bs *BitSwap) getBlock(k u.Key, p *peer.Peer, timeout time.Duration) ([]byte, error) {
	mes := new(PBMessage)
	mes.Id = proto.Uint64(swarm.GenerateID())
	mes.Key = proto.String(k)
	typ := PBMessage_GET_BLOCK
	mes.Type = &typ

	after := time.After(timeout)
	resp := bs.listener.Listen(mes.GetId(), 1, timeout)
	smes := swarm.NewMessage(p, mes)
	bs.meschan.Outgoing <- smes

	select {
	case resp_mes := <-resp:
	case <-after:
		u.PErr("getBlock for '%s' timed out.", k)
		return nil, u.ErrTimeout
	}
}

// HaveBlock announces the existance of a block to BitSwap, potentially sending
// it to peers (Partners) whose WantLists include it.
func (bs *BitSwap) HaveBlock(k u.Key) (*blocks.Block, error) {
	return nil, errors.New("not implemented")
}

func (bs *BitSwap) handleMessages() {
	for {
		select {
		case mes := bs.meschan.Incoming:
		case <-bs.haltChan:
		}
	}
}
