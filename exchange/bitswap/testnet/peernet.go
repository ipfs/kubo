package bitswap

import (
	"math"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	mockpeernet "github.com/jbenet/go-ipfs/net/mock"
	peer "github.com/jbenet/go-ipfs/peer"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
	delay "github.com/jbenet/go-ipfs/util/delay"
)

type peernet struct {
	mockpeernet.Mocknet
}

func LimitedStreamNetWithDelay(ctx context.Context, n int, d delay.D) (Network, error) {
	net, err := mockpeernet.FullMeshLinked(ctx, n)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	net.SetLinkDefaults(mockpeernet.LinkOptions{
		Latency:   d.Get(),
		Bandwidth: math.MaxInt32, // TODO inject
	})
	return &peernet{net}, nil
}

func (pn *peernet) Adapter(p peer.Peer) bsnet.BitSwapNetwork {
	client, err := pn.Mocknet.AddPeer(p.ID())
	if err != nil {
		panic(err.Error())
	}
	return bsnet.NewFromIpfsNetwork(client)
}

func (pn *peernet) HasPeer(p peer.Peer) bool {
	for _, member := range pn.Mocknet.Peers() {
		if p.ID().Equal(member.ID()) {
			return true
		}
	}
	return false
}

func (pn *peernet) SendMessage(
	ctx context.Context,
	from peer.Peer,
	to peer.Peer,
	message bsmsg.BitSwapMessage) error {
	return errors.New("SendMessage is not used by this mock implementation")
}

func (pn *peernet) SendRequest(
	ctx context.Context,
	from peer.Peer,
	to peer.Peer,
	message bsmsg.BitSwapMessage) (
	incoming bsmsg.BitSwapMessage, err error) {
	return nil, errors.New("SendMessage is not used by this mock implementation")
}

var _ Network = &peernet{}
