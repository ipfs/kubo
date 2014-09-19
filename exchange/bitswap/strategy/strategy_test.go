package strategy

import (
	"strings"
	"testing"

	message "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	peer "github.com/jbenet/go-ipfs/peer"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
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

func TestConsistentAccounting(t *testing.T) {
	sender := newPeerAndStrategist("Ernie")
	receiver := newPeerAndStrategist("Bert")

	// Send messages from Ernie to Bert
	for i := 0; i < 1000; i++ {

		m := message.New()
		content := []string{"this", "is", "message", "i"}
		m.AppendBlock(testutil.NewBlockOrFail(t, strings.Join(content, " ")))

		sender.MessageSent(receiver.Peer, m)
		receiver.MessageReceived(sender.Peer, m)
	}

	// Ensure sender records the change
	if sender.NumBytesSentTo(receiver.Peer) == 0 {
		t.Fatal("Sent bytes were not recorded")
	}

	// Ensure sender and receiver have the same values
	if sender.NumBytesSentTo(receiver.Peer) != receiver.NumBytesReceivedFrom(sender.Peer) {
		t.Fatal("Inconsistent book-keeping. Strategies don't agree")
	}

	// Ensure sender didn't record receving anything. And that the receiver
	// didn't record sending anything
	if receiver.NumBytesSentTo(sender.Peer) != 0 || sender.NumBytesReceivedFrom(receiver.Peer) != 0 {
		t.Fatal("Bert didn't send bytes to Ernie")
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
