package strategy

import (
	"testing"

	message "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	"github.com/jbenet/go-ipfs/peer"
	"github.com/jbenet/go-ipfs/util/testutil"
)

type peerAndStrategist struct {
	*peer.Peer
	Strategy
}

func newPeerAndStrategist(idStr string) peerAndStrategist {
	return peerAndStrategist{
		Peer:     &peer.Peer{ID: peer.ID(idStr)},
		Strategy: New(),
	}
}

func TestBlockRecordedAsWantedAfterMessageReceived(t *testing.T) {
	beggar := newPeerAndStrategist("can't be chooser")
	chooser := newPeerAndStrategist("chooses JIF")

	block := testutil.NewBlockOrFail(t, "data wanted by beggar")

	messageFromBeggarToChooser := message.New()
	messageFromBeggarToChooser.AppendWanted(block.Key())

	chooser.MessageReceived(beggar.Peer, messageFromBeggarToChooser)
	// for this test, doesn't matter if you record that beggar sent

	if !chooser.BlockIsWantedByPeer(block.Key(), beggar.Peer) {
		t.Fatal("chooser failed to record that beggar wants block")
	}
}

func TestPeerIsAddedToPeersWhenMessageReceivedOrSent(t *testing.T) {

	sanfrancisco := newPeerAndStrategist("sf")
	seattle := newPeerAndStrategist("sea")

	m := message.New()

	sanfrancisco.MessageSent(seattle.Peer, m)
	seattle.MessageReceived(sanfrancisco.Peer, m)

	if seattle.Peer.Key() == sanfrancisco.Peer.Key() {
		t.Fatal("Sanity Check: Peers have same Key!")
	}

	if !peerIsPartner(seattle.Peer, sanfrancisco.Strategy) {
		t.Fatal("Peer wasn't added as a Partner")
	}

	if !peerIsPartner(sanfrancisco.Peer, seattle.Strategy) {
		t.Fatal("Peer wasn't added as a Partner")
	}
}

func peerIsPartner(p *peer.Peer, s Strategy) bool {
	for _, partner := range s.Peers() {
		if partner.Key() == p.Key() {
			return true
		}
	}
	return false
}
