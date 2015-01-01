package network

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	host "github.com/jbenet/go-ipfs/p2p/host"
	inet "github.com/jbenet/go-ipfs/p2p/net"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	routing "github.com/jbenet/go-ipfs/routing"
	util "github.com/jbenet/go-ipfs/util"
)

var log = util.Logger("bitswap_network")

// NewFromIpfsHost returns a BitSwapNetwork supported by underlying IPFS host
func NewFromIpfsHost(host host.Host, r routing.IpfsRouting) BitSwapNetwork {
	bitswapNetwork := impl{
		host:    host,
		routing: r,
	}
	host.SetStreamHandler(ProtocolBitswap, bitswapNetwork.handleNewStream)
	return &bitswapNetwork
}

// impl transforms the ipfs network interface, which sends and receives
// NetMessage objects, into the bitswap network interface.
type impl struct {
	host    host.Host
	routing routing.IpfsRouting

	// inbound messages from the network are forwarded to the receiver
	receiver Receiver
}

func (bsnet *impl) DialPeer(ctx context.Context, p peer.ID) error {
	return bsnet.host.Connect(ctx, peer.PeerInfo{ID: p})
}

func (bsnet *impl) SendMessage(
	ctx context.Context,
	p peer.ID,
	outgoing bsmsg.BitSwapMessage) error {

	s, err := bsnet.host.NewStream(ProtocolBitswap, p)
	if err != nil {
		return err
	}
	defer s.Close()

	return outgoing.ToNet(s)
}

func (bsnet *impl) SendRequest(
	ctx context.Context,
	p peer.ID,
	outgoing bsmsg.BitSwapMessage) (bsmsg.BitSwapMessage, error) {

	s, err := bsnet.host.NewStream(ProtocolBitswap, p)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	if err := outgoing.ToNet(s); err != nil {
		return nil, err
	}

	return bsmsg.FromNet(s)
}

func (bsnet *impl) SetDelegate(r Receiver) {
	bsnet.receiver = r
}

// FindProvidersAsync returns a channel of providers for the given key
func (bsnet *impl) FindProvidersAsync(ctx context.Context, k util.Key, max int) <-chan peer.ID {
	out := make(chan peer.ID)
	go func() {
		defer close(out)
		providers := bsnet.routing.FindProvidersAsync(ctx, k, max)
		for info := range providers {
			bsnet.host.Peerstore().AddAddresses(info.ID, info.Addrs)
			select {
			case <-ctx.Done():
			case out <- info.ID:
			}
		}
	}()
	return out
}

// Provide provides the key to the network
func (bsnet *impl) Provide(ctx context.Context, k util.Key) error {
	return bsnet.routing.Provide(ctx, k)
}

// handleNewStream receives a new stream from the network.
func (bsnet *impl) handleNewStream(s inet.Stream) {

	if bsnet.receiver == nil {
		return
	}

	go func() {
		defer s.Close()

		received, err := bsmsg.FromNet(s)
		if err != nil {
			go bsnet.receiver.ReceiveError(err)
			return
		}

		p := s.Conn().RemotePeer()
		ctx := context.Background()
		bsnet.receiver.ReceiveMessage(ctx, p, received)
	}()

}
