package coremock

import (
	"context"

	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/node/libp2p"
)

// NewMockNode constructs an IpfsNode for use in tests.
func NewMockNode() (*core.IpfsNode, error) {
	ctx := context.Background()

	// effectively offline, only peer in its network
	api, err := coreapi.New(coreapi.Ctx(ctx),
		coreapi.Online(),
		coreapi.Override(coreapi.Libp2pHost, libp2p.MockHost),
		coreapi.Provide(mocknet.New))

	if err != nil {
		return nil, err
	}

	return api.Node(), nil
}
