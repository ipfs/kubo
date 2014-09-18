package strategy

import (
	"testing"

	message "github.com/jbenet/go-ipfs/bitswap/message"
	"github.com/jbenet/go-ipfs/peer"
)

type peerAndStrategist struct {
	*peer.Peer
	Strategist
}

func newPeerAndStrategist(idStr string) peerAndStrategist {
	return peerAndStrategist{
		Peer:       &peer.Peer{ID: peer.ID(idStr)},
		Strategist: New(),
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

	if !peerIsPartner(seattle.Peer, sanfrancisco.Strategist) {
		t.Fatal("Peer wasn't added as a Partner")
	}

	if !peerIsPartner(sanfrancisco.Peer, seattle.Strategist) {
		t.Fatal("Peer wasn't added as a Partner")
	}
}

func peerIsPartner(p *peer.Peer, s Strategist) bool {
	for _, partner := range s.Peers() {
		if partner.Key() == p.Key() {
			return true
		}
	}
	return false
}
