package strategy

import (
	"errors"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	bsmsg "github.com/jbenet/go-ipfs/bitswap/message"
	"github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// TODO declare thread-safe datastore
func New(d ds.Datastore) Strategist {
	return &strategist{
		datastore: d,
		peers:     ledgerMap{},
	}
}

type strategist struct {
	datastore ds.Datastore // FIXME(brian): enforce thread-safe datastore

	peers ledgerMap
}

// Peers returns a list of this instance is connected to
func (s *strategist) Peers() []*peer.Peer {
	response := make([]*peer.Peer, 0) // TODO
	return response
}

func (s *strategist) IsWantedByPeer(u.Key, *peer.Peer) bool {
	return true // TODO
}

func (s *strategist) ShouldSendToPeer(u.Key, *peer.Peer) bool {
	return true // TODO
}

func (s *strategist) Seed(int64) {
	// TODO
}

func (s *strategist) MessageReceived(*peer.Peer, bsmsg.BitSwapMessage) error {
	// TODO add peer to partners if doesn't already exist.
	// TODO initialize ledger for peer if doesn't already exist
	// TODO get wantlist from message and update contents in local wantlist for peer
	// TODO acknowledge receipt of blocks and do accounting in ledger
	return errors.New("TODO")
}

func (s *strategist) MessageSent(*peer.Peer, bsmsg.BitSwapMessage) error {
	// TODO add peer to partners if doesn't already exist.
	// TODO initialize ledger for peer if doesn't already exist
	// TODO add block to my wantlist
	// TODO acknowledge receipt of blocks and do accounting in ledger
	return errors.New("TODO")
}
