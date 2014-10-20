package bitswap

import (
	"bytes"
	"errors"
	"fmt"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	peer "github.com/jbenet/go-ipfs/peer"
	"github.com/jbenet/go-ipfs/util"
)

type Network interface {
	Adapter(peer.Peer) bsnet.Adapter

	HasPeer(peer.Peer) bool

	SendMessage(
		ctx context.Context,
		from peer.Peer,
		to peer.Peer,
		message bsmsg.BitSwapMessage) error

	SendRequest(
		ctx context.Context,
		from peer.Peer,
		to peer.Peer,
		message bsmsg.BitSwapMessage) (
		incoming bsmsg.BitSwapMessage, err error)
}

// network impl

func VirtualNetwork() Network {
	return &network{
		clients: make(map[util.Key]bsnet.Receiver),
	}
}

type network struct {
	clients map[util.Key]bsnet.Receiver
}

func (n *network) Adapter(p peer.Peer) bsnet.Adapter {
	client := &networkClient{
		local:   p,
		network: n,
	}
	n.clients[p.Key()] = client
	return client
}

func (n *network) HasPeer(p peer.Peer) bool {
	_, found := n.clients[p.Key()]
	return found
}

// TODO should this be completely asynchronous?
// TODO what does the network layer do with errors received from services?
func (n *network) SendMessage(
	ctx context.Context,
	from peer.Peer,
	to peer.Peer,
	message bsmsg.BitSwapMessage) error {

	receiver, ok := n.clients[to.Key()]
	if !ok {
		return errors.New("Cannot locate peer on network")
	}

	// nb: terminate the context since the context wouldn't actually be passed
	// over the network in a real scenario

	go n.deliver(receiver, from, message)

	return nil
}

func (n *network) deliver(
	r bsnet.Receiver, from peer.Peer, message bsmsg.BitSwapMessage) error {
	if message == nil || from == nil {
		return errors.New("Invalid input")
	}

	nextPeer, nextMsg := r.ReceiveMessage(context.TODO(), from, message)

	if (nextPeer == nil && nextMsg != nil) || (nextMsg == nil && nextPeer != nil) {
		return errors.New("Malformed client request")
	}

	if nextPeer == nil && nextMsg == nil {
		return nil
	}

	nextReceiver, ok := n.clients[nextPeer.Key()]
	if !ok {
		return errors.New("Cannot locate peer on network")
	}
	go n.deliver(nextReceiver, nextPeer, nextMsg)
	return nil
}

var NoResponse = errors.New("No response received from the receiver")

// TODO
func (n *network) SendRequest(
	ctx context.Context,
	from peer.Peer,
	to peer.Peer,
	message bsmsg.BitSwapMessage) (
	incoming bsmsg.BitSwapMessage, err error) {

	r, ok := n.clients[to.Key()]
	if !ok {
		return nil, errors.New("Cannot locate peer on network")
	}
	nextPeer, nextMsg := r.ReceiveMessage(context.TODO(), from, message)

	// TODO dedupe code
	if (nextPeer == nil && nextMsg != nil) || (nextMsg == nil && nextPeer != nil) {
		r.ReceiveError(errors.New("Malformed client request"))
		return nil, nil
	}

	// TODO dedupe code
	if nextPeer == nil && nextMsg == nil {
		return nil, nil
	}

	// TODO test when receiver doesn't immediately respond to the initiator of the request
	if !bytes.Equal(nextPeer.ID(), from.ID()) {
		go func() {
			nextReceiver, ok := n.clients[nextPeer.Key()]
			if !ok {
				// TODO log the error?
			}
			n.deliver(nextReceiver, nextPeer, nextMsg)
		}()
		return nil, nil
	}
	return nextMsg, nil
}

type networkClient struct {
	local peer.Peer
	bsnet.Receiver
	network Network
}

func (nc *networkClient) SendMessage(
	ctx context.Context,
	to peer.Peer,
	message bsmsg.BitSwapMessage) error {
	return nc.network.SendMessage(ctx, nc.local, to, message)
}

func (nc *networkClient) SendRequest(
	ctx context.Context,
	to peer.Peer,
	message bsmsg.BitSwapMessage) (incoming bsmsg.BitSwapMessage, err error) {
	return nc.network.SendRequest(ctx, nc.local, to, message)
}

func (nc *networkClient) DialPeer(p peer.Peer) error {
	// no need to do anything because dialing isn't a thing in this test net.
	if !nc.network.HasPeer(p) {
		return fmt.Errorf("Peer not in network: %s", p)
	}
	return nil
}

func (nc *networkClient) SetDelegate(r bsnet.Receiver) {
	nc.Receiver = r
}
