package bitswap

import (
	"math"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	mockpeernet "github.com/jbenet/go-ipfs/net/mock"
	peer "github.com/jbenet/go-ipfs/peer"
	mockrouting "github.com/jbenet/go-ipfs/routing/mock"
	delay "github.com/jbenet/go-ipfs/util/delay"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

type peernet struct {
	mockpeernet.Mocknet
	routingserver mockrouting.Server
}

func StreamNetWithDelay(
	ctx context.Context,
	rs mockrouting.Server,
	d delay.D) (Network, error) {

	net := mockpeernet.New(ctx)
	net.SetLinkDefaults(mockpeernet.LinkOptions{
		Latency:   d.Get(),
		Bandwidth: math.MaxInt32, // TODO inject
	})
	return &peernet{net, rs}, nil
}

func (pn *peernet) Adapter(p testutil.Peer) bsnet.BitSwapNetwork {
	peers := pn.Mocknet.Peers()
	client, err := pn.Mocknet.AddPeer(p.PrivateKey(), p.Address())
	if err != nil {
		panic(err.Error())
	}
	for _, other := range peers {
		pn.Mocknet.LinkPeers(p.ID(), other)
	}
	routing := pn.routingserver.Client(peer.PeerInfo{ID: p.ID()})
	return bsnet.NewFromIpfsNetwork(client, routing)
}

func (pn *peernet) HasPeer(p peer.ID) bool {
	for _, member := range pn.Mocknet.Peers() {
		if p == member {
			return true
		}
	}
	return false
}

var _ Network = &peernet{}
