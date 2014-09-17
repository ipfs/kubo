package network

import (
	"testing"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	bsmsg "github.com/jbenet/go-ipfs/bitswap/message"
	peer "github.com/jbenet/go-ipfs/peer"
)

func TestDoesntPanicIfDelegateNotPresent(t *testing.T) {
	fwdr := Forwarder{}
	fwdr.ReceiveMessage(context.Background(), &peer.Peer{}, bsmsg.New())
}

func TestForwardsMessageToDelegate(t *testing.T) {
	fwdr := Forwarder{delegate: &EchoDelegate{}}
	fwdr.ReceiveMessage(context.Background(), &peer.Peer{}, bsmsg.New())
}

type EchoDelegate struct{}

func (d *EchoDelegate) ReceiveMessage(ctx context.Context, p *peer.Peer,
	incoming bsmsg.BitSwapMessage) (*peer.Peer, bsmsg.BitSwapMessage, error) {
	return p, incoming, nil
}
