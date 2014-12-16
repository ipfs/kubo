package strategy

import (
	"strings"
	"testing"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	blocks "github.com/jbenet/go-ipfs/blocks"
	message "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	peer "github.com/jbenet/go-ipfs/peer"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

type peerAndLedgermanager struct {
	peer.Peer
	ls *LedgerManager
}

func newPeerAndLedgermanager(idStr string) peerAndLedgermanager {
	return peerAndLedgermanager{
		Peer: testutil.NewPeerWithIDString(idStr),
		//Strategy: New(true),
		ls: NewLedgerManager(nil, context.TODO()),
	}
}

func TestConsistentAccounting(t *testing.T) {
	sender := newPeerAndLedgermanager("Ernie")
	receiver := newPeerAndLedgermanager("Bert")

	// Send messages from Ernie to Bert
	for i := 0; i < 1000; i++ {

		m := message.New()
		content := []string{"this", "is", "message", "i"}
		m.AddBlock(blocks.NewBlock([]byte(strings.Join(content, " "))))

		sender.ls.MessageSent(receiver.Peer, m)
		receiver.ls.MessageReceived(sender.Peer, m)
	}

	// Ensure sender records the change
	if sender.ls.NumBytesSentTo(receiver.Peer) == 0 {
		t.Fatal("Sent bytes were not recorded")
	}

	// Ensure sender and receiver have the same values
	if sender.ls.NumBytesSentTo(receiver.Peer) != receiver.ls.NumBytesReceivedFrom(sender.Peer) {
		t.Fatal("Inconsistent book-keeping. Strategies don't agree")
	}

	// Ensure sender didn't record receving anything. And that the receiver
	// didn't record sending anything
	if receiver.ls.NumBytesSentTo(sender.Peer) != 0 || sender.ls.NumBytesReceivedFrom(receiver.Peer) != 0 {
		t.Fatal("Bert didn't send bytes to Ernie")
	}
}

func TestBlockRecordedAsWantedAfterMessageReceived(t *testing.T) {
	beggar := newPeerAndLedgermanager("can't be chooser")
	chooser := newPeerAndLedgermanager("chooses JIF")

	block := blocks.NewBlock([]byte("data wanted by beggar"))

	messageFromBeggarToChooser := message.New()
	messageFromBeggarToChooser.AddEntry(block.Key(), 1, false)

	chooser.ls.MessageReceived(beggar.Peer, messageFromBeggarToChooser)
	// for this test, doesn't matter if you record that beggar sent

	if !chooser.ls.BlockIsWantedByPeer(block.Key(), beggar.Peer) {
		t.Fatal("chooser failed to record that beggar wants block")
	}
}

func TestPeerIsAddedToPeersWhenMessageReceivedOrSent(t *testing.T) {

	sanfrancisco := newPeerAndLedgermanager("sf")
	seattle := newPeerAndLedgermanager("sea")

	m := message.New()

	sanfrancisco.ls.MessageSent(seattle.Peer, m)
	seattle.ls.MessageReceived(sanfrancisco.Peer, m)

	if seattle.Peer.Key() == sanfrancisco.Peer.Key() {
		t.Fatal("Sanity Check: Peers have same Key!")
	}

	if !peerIsPartner(seattle.Peer, sanfrancisco.ls) {
		t.Fatal("Peer wasn't added as a Partner")
	}

	if !peerIsPartner(sanfrancisco.Peer, seattle.ls) {
		t.Fatal("Peer wasn't added as a Partner")
	}
}

func peerIsPartner(p peer.Peer, ls *LedgerManager) bool {
	for _, partner := range ls.Peers() {
		if partner.Key() == p.Key() {
			return true
		}
	}
	return false
}
