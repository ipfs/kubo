package testutil

import (
	"testing"

	bhost "github.com/jbenet/go-ipfs/p2p/host/basic"
	inet "github.com/jbenet/go-ipfs/p2p/net"
	swarm "github.com/jbenet/go-ipfs/p2p/net/swarm"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	tu "github.com/jbenet/go-ipfs/util/testutil"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

func GenSwarmNetwork(t *testing.T, ctx context.Context) *swarm.Network {
	p := tu.RandPeerNetParamsOrFatal(t)
	ps := peer.NewPeerstore()
	ps.AddPubKey(p.ID, p.PubKey)
	ps.AddPrivKey(p.ID, p.PrivKey)
	n, err := swarm.NewNetwork(ctx, []ma.Multiaddr{p.Addr}, p.ID, ps)
	if err != nil {
		t.Fatal(err)
	}
	ps.AddAddresses(p.ID, n.ListenAddresses())
	return n
}

func DivulgeAddresses(a, b inet.Network) {
	id := a.LocalPeer()
	addrs := a.Peerstore().Addresses(id)
	b.Peerstore().AddAddresses(id, addrs)
}

func GenHostSwarm(t *testing.T, ctx context.Context) *bhost.BasicHost {
	n := GenSwarmNetwork(t, ctx)
	return bhost.New(n)
}
