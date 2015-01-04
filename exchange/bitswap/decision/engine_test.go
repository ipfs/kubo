package decision

import (
	"strings"
	"testing"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	sync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"

	blocks "github.com/jbenet/go-ipfs/blocks"
	blockstore "github.com/jbenet/go-ipfs/blocks/blockstore"
	message "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
)

type peerAndEngine struct {
	Peer   peer.ID
	Engine *Engine
}

func newPeerAndLedgermanager(idStr string) peerAndEngine {
	return peerAndEngine{
		Peer: peer.ID(idStr),
		//Strategy: New(true),
		Engine: NewEngine(context.TODO(),
			blockstore.NewBlockstore(sync.MutexWrap(ds.NewMapDatastore()))),
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

		sender.Engine.MessageSent(receiver.Peer, m)
		receiver.Engine.MessageReceived(sender.Peer, m)
	}

	// Ensure sender records the change
	if sender.Engine.numBytesSentTo(receiver.Peer) == 0 {
		t.Fatal("Sent bytes were not recorded")
	}

	// Ensure sender and receiver have the same values
	if sender.Engine.numBytesSentTo(receiver.Peer) != receiver.Engine.numBytesReceivedFrom(sender.Peer) {
		t.Fatal("Inconsistent book-keeping. Strategies don't agree")
	}

	// Ensure sender didn't record receving anything. And that the receiver
	// didn't record sending anything
	if receiver.Engine.numBytesSentTo(sender.Peer) != 0 || sender.Engine.numBytesReceivedFrom(receiver.Peer) != 0 {
		t.Fatal("Bert didn't send bytes to Ernie")
	}
}

func TestPeerIsAddedToPeersWhenMessageReceivedOrSent(t *testing.T) {

	sanfrancisco := newPeerAndLedgermanager("sf")
	seattle := newPeerAndLedgermanager("sea")

	m := message.New()

	sanfrancisco.Engine.MessageSent(seattle.Peer, m)
	seattle.Engine.MessageReceived(sanfrancisco.Peer, m)

	if seattle.Peer == sanfrancisco.Peer {
		t.Fatal("Sanity Check: Peers have same Key!")
	}

	if !peerIsPartner(seattle.Peer, sanfrancisco.Engine) {
		t.Fatal("Peer wasn't added as a Partner")
	}

	if !peerIsPartner(sanfrancisco.Peer, seattle.Engine) {
		t.Fatal("Peer wasn't added as a Partner")
	}
}

func peerIsPartner(p peer.ID, e *Engine) bool {
	for _, partner := range e.Peers() {
		if partner == p {
			return true
		}
	}
	return false
}
