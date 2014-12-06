package network

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	inet "github.com/jbenet/go-ipfs/net"
	netmsg "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"
	util "github.com/jbenet/go-ipfs/util"
)

var log = util.Logger("bitswap_network")

// NewFromIpfsNetwork returns a BitSwapNetwork supported by underlying IPFS
// Dialer & Service
func NewFromIpfsNetwork(s inet.Service, dialer inet.Dialer) BitSwapNetwork {
	bitswapNetwork := impl{
		service: s,
		dialer:  dialer,
	}
	s.SetHandler(&bitswapNetwork)
	return &bitswapNetwork
}

// impl transforms the ipfs network interface, which sends and receives
// NetMessage objects, into the bitswap network interface.
type impl struct {
	service inet.Service
	dialer  inet.Dialer

	// inbound messages from the network are forwarded to the receiver
	receiver Receiver
}

// HandleMessage marshals and unmarshals net messages, forwarding them to the
// BitSwapMessage receiver
func (bsnet *impl) HandleMessage(
	ctx context.Context, incoming netmsg.NetMessage) netmsg.NetMessage {

	if bsnet.receiver == nil {
		return nil
	}

	received, err := bsmsg.FromNet(incoming)
	if err != nil {
		go bsnet.receiver.ReceiveError(err)
		return nil
	}

	bsnet.receiver.ReceiveMessage(ctx, incoming.Peer(), received)
	return nil
}

func (bsnet *impl) DialPeer(ctx context.Context, p peer.Peer) error {
	return bsnet.dialer.DialPeer(ctx, p)
}

func (bsnet *impl) SendMessage(
	ctx context.Context,
	p peer.Peer,
	outgoing bsmsg.BitSwapMessage) error {

	nmsg, err := outgoing.ToNet(p)
	if err != nil {
		return err
	}
	return bsnet.service.SendMessage(ctx, nmsg)
}

func (bsnet *impl) SendRequest(
	ctx context.Context,
	p peer.Peer,
	outgoing bsmsg.BitSwapMessage) (bsmsg.BitSwapMessage, error) {

	outgoingMsg, err := outgoing.ToNet(p)
	if err != nil {
		return nil, err
	}
	incomingMsg, err := bsnet.service.SendRequest(ctx, outgoingMsg)
	if err != nil {
		return nil, err
	}
	return bsmsg.FromNet(incomingMsg)
}

func (bsnet *impl) SetDelegate(r Receiver) {
	bsnet.receiver = r
}
