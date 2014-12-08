package bitswap

import (
	"sync"
	"testing"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	blocks "github.com/jbenet/go-ipfs/blocks"
	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	peer "github.com/jbenet/go-ipfs/peer"
	"github.com/jbenet/go-ipfs/peer/mock"
)

func TestSendRequestToCooperativePeer(t *testing.T) {
	net := VirtualNetwork()

	idOfRecipient := []byte("recipient")

	t.Log("Get two network adapters")

	initiator := net.Adapter(mockpeer.WithIDString("initiator"))
	recipient := net.Adapter(mockpeer.WithID(idOfRecipient))

	expectedStr := "response from recipient"
	recipient.SetDelegate(lambda(func(
		ctx context.Context,
		from peer.Peer,
		incoming bsmsg.BitSwapMessage) (
		peer.Peer, bsmsg.BitSwapMessage) {

		t.Log("Recipient received a message from the network")

		// TODO test contents of incoming message

		m := bsmsg.New()
		m.AddBlock(blocks.NewBlock([]byte(expectedStr)))

		return from, m
	}))

	t.Log("Build a message and send a synchronous request to recipient")

	message := bsmsg.New()
	message.AddBlock(blocks.NewBlock([]byte("data")))
	response, err := initiator.SendRequest(
		context.Background(), mockpeer.WithID(idOfRecipient), message)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("Check the contents of the response from recipient")

	for _, blockFromRecipient := range response.Blocks() {
		if string(blockFromRecipient.Data) == expectedStr {
			return
		}
	}
	t.Fatal("Should have returned after finding expected block data")
}

func TestSendMessageAsyncButWaitForResponse(t *testing.T) {
	net := VirtualNetwork()
	idOfResponder := []byte("responder")
	waiter := net.Adapter(mockpeer.WithIDString("waiter"))
	responder := net.Adapter(mockpeer.WithID(idOfResponder))

	var wg sync.WaitGroup

	wg.Add(1)

	expectedStr := "received async"

	responder.SetDelegate(lambda(func(
		ctx context.Context,
		fromWaiter peer.Peer,
		msgFromWaiter bsmsg.BitSwapMessage) (
		peer.Peer, bsmsg.BitSwapMessage) {

		msgToWaiter := bsmsg.New()
		msgToWaiter.AddBlock(blocks.NewBlock([]byte(expectedStr)))

		return fromWaiter, msgToWaiter
	}))

	waiter.SetDelegate(lambda(func(
		ctx context.Context,
		fromResponder peer.Peer,
		msgFromResponder bsmsg.BitSwapMessage) (
		peer.Peer, bsmsg.BitSwapMessage) {

		// TODO assert that this came from the correct peer and that the message contents are as expected
		ok := false
		for _, b := range msgFromResponder.Blocks() {
			if string(b.Data) == expectedStr {
				wg.Done()
				ok = true
			}
		}

		if !ok {
			t.Fatal("Message not received from the responder")

		}
		return nil, nil
	}))

	messageSentAsync := bsmsg.New()
	messageSentAsync.AddBlock(blocks.NewBlock([]byte("data")))
	errSending := waiter.SendMessage(
		context.Background(), mockpeer.WithID(idOfResponder), messageSentAsync)
	if errSending != nil {
		t.Fatal(errSending)
	}

	wg.Wait() // until waiter delegate function is executed
}

type receiverFunc func(ctx context.Context, p peer.Peer,
	incoming bsmsg.BitSwapMessage) (peer.Peer, bsmsg.BitSwapMessage)

// lambda returns a Receiver instance given a receiver function
func lambda(f receiverFunc) bsnet.Receiver {
	return &lambdaImpl{
		f: f,
	}
}

type lambdaImpl struct {
	f func(ctx context.Context, p peer.Peer, incoming bsmsg.BitSwapMessage) (
		peer.Peer, bsmsg.BitSwapMessage)
}

func (lam *lambdaImpl) ReceiveMessage(ctx context.Context,
	p peer.Peer, incoming bsmsg.BitSwapMessage) (
	peer.Peer, bsmsg.BitSwapMessage) {
	return lam.f(ctx, p, incoming)
}

func (lam *lambdaImpl) ReceiveError(err error) {
	// TODO log error
}
