package coremock

import (
	"context"

	commands "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/repo"

	pstore "gx/ipfs/QmNVzTd29YipdnuXB67b2VnFSerudTVS3ZrxyBhNxcdvAg/go-libp2p-peerstore"
	config "gx/ipfs/QmNqGkQ53mLMJ2Qe7WmYwv2Vo9QjcYTfsM4bcDNvfS6AXV/go-ipfs-config"
	host "gx/ipfs/QmUR3nQ8nSP7vUnzTvz2kLxq4ncud6eC2s4BAEC8uiAsde/go-libp2p-host"
	peer "gx/ipfs/QmUz7HmU4ws577TMEVNoTjNghVvq2jXnxnVhbkNXKPKpgK/go-libp2p-peer"
	libp2p "gx/ipfs/QmW1qc6tENFpuvv3VfjSHRz1GiVYGiQa13nnEMJBaJDBqW/go-libp2p"
	mocknet "gx/ipfs/QmW1qc6tENFpuvv3VfjSHRz1GiVYGiQa13nnEMJBaJDBqW/go-libp2p/p2p/net/mock"
	datastore "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	syncds "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore/sync"
	testutil "gx/ipfs/QmbtxgRVMQS2fZpuwowZRQUg3QNri9yuHSFGfXUrqoSABU/go-testutil"
)

// NewMockNode constructs an IpfsNode for use in tests.
func NewMockNode() (*core.IpfsNode, error) {
	ctx := context.Background()

	// effectively offline, only peer in its network
	return core.NewNode(ctx, &core.BuildCfg{
		Online: true,
		Host:   MockHostOption(mocknet.New(ctx)),
	})
}

func MockHostOption(mn mocknet.Mocknet) core.HostOption {
	return func(ctx context.Context, id peer.ID, ps pstore.Peerstore, _ ...libp2p.Option) (host.Host, error) {
		return mn.AddPeerWithPeerstore(id, ps)
	}
}

func MockCmdsCtx() (commands.Context, error) {
	// Generate Identity
	ident, err := testutil.RandIdentity()
	if err != nil {
		return commands.Context{}, err
	}
	p := ident.ID()

	conf := config.Config{
		Identity: config.Identity{
			PeerID: p.String(),
		},
	}

	r := &repo.Mock{
		D: syncds.MutexWrap(datastore.NewMapDatastore()),
		C: conf,
	}

	node, err := core.NewNode(context.Background(), &core.BuildCfg{
		Repo: r,
	})
	if err != nil {
		return commands.Context{}, err
	}

	return commands.Context{
		Online:     true,
		ConfigRoot: "/tmp/.mockipfsconfig",
		LoadConfig: func(path string) (*config.Config, error) {
			return &conf, nil
		},
		ConstructNode: func() (*core.IpfsNode, error) {
			return node, nil
		},
	}, nil
}
