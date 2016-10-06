package coremock

import (
	"net"

	context "context"
	"gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
	syncds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore/sync"

	commands "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/repo"
	config "github.com/ipfs/go-ipfs/repo/config"
	ds2 "github.com/ipfs/go-ipfs/thirdparty/datastore2"
	testutil "github.com/ipfs/go-ipfs/thirdparty/testutil"
	metrics "gx/ipfs/QmWpTXhTkpoCDEm9twJd5Rc9jFwy61emzxneeJzrVMfjGF/go-libp2p-metrics"
	pstore "gx/ipfs/QmXXCcQ7CLg5a81Ui9TTR35QcR4y7ZyihxwfjqaHfUVcVo/go-libp2p-peerstore"
	mocknet "gx/ipfs/QmcRa2qn6iCmap9bjp8jAwkvYAq13AUfxdY3rrYiaJbLum/go-libp2p/p2p/net/mock"
	host "gx/ipfs/QmdML3R42PRSwnt46jSuEts9bHSqLctVYEjJqMR3UYV8ki/go-libp2p-host"
	peer "gx/ipfs/QmfMmLGoKzCHDN7cGgk64PJr4iipzidDRME8HABSJqvmhC/go-libp2p-peer"
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
	return func(ctx context.Context, id peer.ID, ps pstore.Peerstore, bwr metrics.Reporter, fs []*net.IPNet) (host.Host, error) {
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
		D: ds2.CloserWrap(syncds.MutexWrap(datastore.NewMapDatastore())),
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
