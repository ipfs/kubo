package bitswap

import (
	"sync"
	"testing"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	peer "github.com/jbenet/go-ipfs/peer"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

func TestSendRequestToCooperativePeer(t *testing.T) {
	net := VirtualNetwork()

	idOfRecipient := []byte("recipient")

	t.Log("Get two network adapters")

	initiator := net.Adapter(&peer.Peer{ID: []byte("initiator")})
	recipient := net.Adapter(&peer.Peer{ID: idOfRecipient})

	expectedStr := "response from recipient"
	recipient.SetDelegate(lambda(func(
		ctx context.Context,
		from *peer.Peer,
		incoming bsmsg.BitSwapMessage) (
		*peer.Peer, bsmsg.BitSwapMessage, error) {

		t.Log("Recipient received a message from the network")

		// TODO test contents of incoming message

		m := bsmsg.New()
		m.AppendBlock(testutil.NewBlockOrFail(t, expectedStr))

		return from, m, nil
	}))

	t.Log("Build a message and send a synchronous request to recipient")

	message := bsmsg.New()
	message.AppendBlock(testutil.NewBlockOrFail(t, "data"))
	response, err := initiator.SendRequest(
		context.Background(), &peer.Peer{ID: idOfRecipient}, message)
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
	waiter := net.Adapter(&peer.Peer{ID: []byte("waiter")})
	responder := net.Adapter(&peer.Peer{ID: idOfResponder})

	var wg sync.WaitGroup

	wg.Add(1)

	expectedStr := "received async"

	responder.SetDelegate(lambda(func(
		ctx context.Context,
		fromWaiter *peer.Peer,
		msgFromWaiter bsmsg.BitSwapMessage) (
		*peer.Peer, bsmsg.BitSwapMessage, error) {

		msgToWaiter := bsmsg.New()
		msgToWaiter.AppendBlock(testutil.NewBlockOrFail(t, expectedStr))

		return fromWaiter, msgToWaiter, nil
	}))

	waiter.SetDelegate(lambda(func(
		ctx context.Context,
		fromResponder *peer.Peer,
		msgFromResponder bsmsg.BitSwapMessage) (
		*peer.Peer, bsmsg.BitSwapMessage, error) {

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
		return nil, nil, nil
	}))

	messageSentAsync := bsmsg.New()
	messageSentAsync.AppendBlock(testutil.NewBlockOrFail(t, "data"))
	errSending := waiter.SendMessage(
		context.Background(), &peer.Peer{ID: idOfResponder}, messageSentAsync)
	if errSending != nil {
		t.Fatal(errSending)
	}

	wg.Wait() // until waiter delegate function is executed
}

type receiverFunc func(ctx context.Context, p *peer.Peer,
	incoming bsmsg.BitSwapMessage) (*peer.Peer, bsmsg.BitSwapMessage, error)

// lambda returns a Receiver instance given a receiver function
func lambda(f receiverFunc) bsnet.Receiver {
	return &lambdaImpl{
		f: f,
	}
}

type lambdaImpl struct {
	f func(ctx context.Context, p *peer.Peer,
		incoming bsmsg.BitSwapMessage) (
		*peer.Peer, bsmsg.BitSwapMessage, error)
}

func (lam *lambdaImpl) ReceiveMessage(ctx context.Context,
	p *peer.Peer, incoming bsmsg.BitSwapMessage) (
	*peer.Peer, bsmsg.BitSwapMessage, error) {
	return lam.f(ctx, p, incoming)
}
