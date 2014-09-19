package network

import (
	"errors"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	netmsg "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"
)

// NewSender wraps a network Service to perform translation between
// BitSwapMessage and NetMessage formats. This allows the BitSwap session to
// ignore these details.
func NewNetworkAdapter(s NetworkService, r Receiver) NetworkAdapter {
	adapter := networkAdapter{
		networkService: s,
		receiver:       r,
	}
	s.SetHandler(&adapter)
	return &adapter
}

// networkAdapter implements NetworkAdapter
type networkAdapter struct {
	networkService NetworkService
	receiver       Receiver
}

// HandleMessage marshals and unmarshals net messages, forwarding them to the
// BitSwapMessage receiver
func (adapter *networkAdapter) HandleMessage(
	ctx context.Context, incoming netmsg.NetMessage) (netmsg.NetMessage, error) {

	if adapter.receiver == nil {
		return nil, errors.New("No receiver. NetMessage dropped")
	}

	received, err := bsmsg.FromNet(incoming)
	if err != nil {
		return nil, err
	}

	p, bsmsg, err := adapter.receiver.ReceiveMessage(ctx, incoming.Peer(), received)
	if err != nil {
		return nil, err
	}

	// TODO(brian): put this in a helper function
	if bsmsg == nil || p == nil {
		return nil, nil
	}

	outgoing, err := bsmsg.ToNet(p)
	if err != nil {
		return nil, err
	}

	return outgoing, nil
}

func (adapter *networkAdapter) SendMessage(
	ctx context.Context,
	p *peer.Peer,
	outgoing bsmsg.BitSwapMessage) error {

	nmsg, err := outgoing.ToNet(p)
	if err != nil {
		return err
	}
	return adapter.networkService.SendMessage(ctx, nmsg)
}

func (adapter *networkAdapter) SendRequest(
	ctx context.Context,
	p *peer.Peer,
	outgoing bsmsg.BitSwapMessage) (bsmsg.BitSwapMessage, error) {

	outgoingMsg, err := outgoing.ToNet(p)
	if err != nil {
		return nil, err
	}
	incomingMsg, err := adapter.networkService.SendRequest(ctx, outgoingMsg)
	if err != nil {
		return nil, err
	}
	return bsmsg.FromNet(incomingMsg)
}

func (adapter *networkAdapter) SetDelegate(r Receiver) {
	adapter.receiver = r
}
