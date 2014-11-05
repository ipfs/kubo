package network

import (
	"errors"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/util"

	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	inet "github.com/jbenet/go-ipfs/net"
	netmsg "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"
)

var log = util.Logger("net_message_adapter")

// NetMessageAdapter wraps a NetMessage network service
func NetMessageAdapter(s inet.Service, n inet.Network, r Receiver) Adapter {
	adapter := impl{
		nms:      s,
		net:      n,
		receiver: r,
	}
	s.SetHandler(&adapter)
	return &adapter
}

// implements an Adapter that integrates with a NetMessage network service
type impl struct {
	nms inet.Service
	net inet.Network

	// inbound messages from the network are forwarded to the receiver
	receiver Receiver
}

// HandleMessage marshals and unmarshals net messages, forwarding them to the
// BitSwapMessage receiver
func (adapter *impl) HandleMessage(
	ctx context.Context, incoming netmsg.NetMessage) netmsg.NetMessage {

	if adapter.receiver == nil {
		return nil
	}

	received, err := bsmsg.FromNet(incoming)
	if err != nil {
		go adapter.receiver.ReceiveError(err)
		return nil
	}

	p, bsmsg := adapter.receiver.ReceiveMessage(ctx, incoming.Peer(), received)

	// TODO(brian): put this in a helper function
	if bsmsg == nil || p == nil {
		adapter.receiver.ReceiveError(errors.New("ReceiveMessage returned nil peer or message"))
		return nil
	}

	outgoing, err := bsmsg.ToNet(p)
	if err != nil {
		go adapter.receiver.ReceiveError(err)
		return nil
	}

	log.Debugf("Message size: %d", len(outgoing.Data()))
	return outgoing
}

func (adapter *impl) DialPeer(ctx context.Context, p peer.Peer) error {
	return adapter.net.DialPeer(p)
}

func (adapter *impl) SendMessage(
	ctx context.Context,
	p peer.Peer,
	outgoing bsmsg.BitSwapMessage) error {

	nmsg, err := outgoing.ToNet(p)
	if err != nil {
		return err
	}
	return adapter.nms.SendMessage(ctx, nmsg)
}

func (adapter *impl) SendRequest(
	ctx context.Context,
	p peer.Peer,
	outgoing bsmsg.BitSwapMessage) (bsmsg.BitSwapMessage, error) {

	outgoingMsg, err := outgoing.ToNet(p)
	if err != nil {
		return nil, err
	}
	incomingMsg, err := adapter.nms.SendRequest(ctx, outgoingMsg)
	if err != nil {
		return nil, err
	}
	return bsmsg.FromNet(incomingMsg)
}

func (adapter *impl) SetDelegate(r Receiver) {
	adapter.receiver = r
}
