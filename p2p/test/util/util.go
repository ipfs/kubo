package testutil

import (
	"testing"

	bhost "github.com/jbenet/go-ipfs/p2p/host/basic"
	inet "github.com/jbenet/go-ipfs/p2p/net2"
	swarm "github.com/jbenet/go-ipfs/p2p/net2/swarm"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	tu "github.com/jbenet/go-ipfs/util/testutil"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

func GenSwarmNetwork(t *testing.T, ctx context.Context) *swarm.Network {
	p := tu.RandPeerNetParamsOrFatal(t)
	ps := peer.NewPeerstore()
	ps.AddAddress(p.ID, p.Addr)
	ps.AddPubKey(p.ID, p.PubKey)
	ps.AddPrivKey(p.ID, p.PrivKey)
	n, err := swarm.NewNetwork(ctx, ps.Addresses(p.ID), p.ID, ps)
	if err != nil {
		t.Fatal(err)
	}
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
