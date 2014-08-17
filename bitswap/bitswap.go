package bitswap

import (
	blocks "github.com/jbenet/go-ipfs/blocks"
	peer "github.com/jbenet/go-ipfs/peer"
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

// Ledger stores the data exchange relationship between two peers.
type Ledger struct {

	// Partner is the ID of the remote Peer.
	Partner peer.ID

	// BytesSent counts the number of bytes the local peer sent to Partner
	BytesSent uint64

	// BytesReceived counts the number of bytes local peer received from Partner
	BytesReceived uint64

	// FirstExchnage is the time of the first data exchange.
	FirstExchange *time.Time

	// LastExchange is the time of the last data exchange.
	LastExchange *time.Time

	// WantList is a (bounded, small) set of keys that Partner desires.
	WantList KeySet
}

// LedgerMap lists Ledgers by their Partner key.
type LedgerMap map[u.Key]*Ledger

// BitSwap instances implement the bitswap protocol.
type BitSwap struct {
	// peer is the identity of this (local) node.
	peer *peer.Peer

	// net holds the connections to all peers.
	net swarm.Network

	// datastore is the local database
	// Ledgers of known
	datastore ds.Datastore

	// partners is a map of currently active bitswap relationships.
	// The Ledger has the peer.ID, and the peer connection works through net.
	// Ledgers of known relationships (active or inactive) stored in datastore.
	// Changes to the Ledger should be committed to the datastore.
	partners map[u.Key]*Ledger

	// haveList is the set of keys we have values for. a map for fast lookups.
	// haveList KeySet -- not needed. all values in datastore?

	// wantList is the set of keys we want values for. a map for fast lookups.
	wantList KeySet
}

// NewBitSwap creates a new BitSwap instance. It does not check its parameters.
func NewBitSwap(p *peer.Peer, net swarm.Network, d ds.Datastore) *BitSwap {
	return &BitSwap{
		peer:      p,
		net:       net,
		datastore: d,
		partners:  LedgerMap{},
		wantList:  KeySet{},
	}
}

// GetBlock attempts to retrieve a particular block from peers, within timeout.
func (s *BitSwap) GetBlock(k u.Key, timeout time.Time) (
	*blocks.Block, error) {
	return nil, errors.New("not implemented")
}

// HaveBlock announces the existance of a block to BitSwap, potentially sending
// it to peers (Partners) whose WantLists include it.
func (s *BitSwap) HaveBlock(k u.Key) (*blocks.Block, error) {
	return nil, errors.New("not implemented")
}
