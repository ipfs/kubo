package gen

import (
	"fmt"

	ic "github.com/jbenet/go-ipfs/p2p/crypto"
	host "github.com/jbenet/go-ipfs/p2p/host"
	bhost "github.com/jbenet/go-ipfs/p2p/host/basic"
	swarm "github.com/jbenet/go-ipfs/p2p/net/swarm"
	peer "github.com/jbenet/go-ipfs/p2p/peer"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

type NetGenerator interface {
	AddPeer(pri ic.PrivKey, pid peer.ID) (host.Host, error)
}

type realNetGenerator struct {
	beginPort int
	ctx       context.Context
}

func NewNetGenerator(ctx context.Context) NetGenerator {
	return &realNetGenerator{
		beginPort: 10000,
	}
}

func (rng *realNetGenerator) AddPeer(pri ic.PrivKey, pid peer.ID) (host.Host, error) {
	pstore := peer.NewPeerstore()
	err := pstore.AddPrivKey(pid, pri)
	if err != nil {
		return nil, err
	}
	addr := ma.StringCast(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", rng.beginPort))
	net, err := swarm.NewNetwork(rng.ctx, []ma.Multiaddr{addr}, pid, pstore)
	if err != nil {
		return nil, err
	}

	rng.beginPort++

	return bhost.New(net), nil
}
