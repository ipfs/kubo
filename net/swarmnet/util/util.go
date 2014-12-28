package testutil

import (
	"testing"

	inet "github.com/jbenet/go-ipfs/net"
	sn "github.com/jbenet/go-ipfs/net/swarmnet"
	peer "github.com/jbenet/go-ipfs/peer"
	tu "github.com/jbenet/go-ipfs/util/testutil"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

func GenNetwork(t *testing.T, ctx context.Context) *sn.Network {
	p := tu.RandPeerNetParamsOrFatal(t)
	ps := peer.NewPeerstore()
	ps.AddAddress(p.ID, p.Addr)
	ps.AddPubKey(p.ID, p.PubKey)
	ps.AddPrivKey(p.ID, p.PrivKey)
	n, err := sn.NewNetwork(ctx, ps.Addresses(p.ID), p.ID, ps)
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
