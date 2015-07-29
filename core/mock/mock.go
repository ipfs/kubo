package coremock

import (
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	syncds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	"github.com/ipfs/go-ipfs/blocks/blockstore"
	blockservice "github.com/ipfs/go-ipfs/blockservice"
	commands "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/exchange/offline"
	mdag "github.com/ipfs/go-ipfs/merkledag"
	nsys "github.com/ipfs/go-ipfs/namesys"
	mocknet "github.com/ipfs/go-ipfs/p2p/net/mock"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	path "github.com/ipfs/go-ipfs/path"
	pin "github.com/ipfs/go-ipfs/pin"
	"github.com/ipfs/go-ipfs/repo"
	config "github.com/ipfs/go-ipfs/repo/config"
	offrt "github.com/ipfs/go-ipfs/routing/offline"
	ds2 "github.com/ipfs/go-ipfs/util/datastore2"
	testutil "github.com/ipfs/go-ipfs/util/testutil"
)

// TODO this is super sketch. Deprecate and initialize one that shares code
// with the actual core constructor. Lots of fields aren't initialized.
// "This is as good as broken." --- is it?

// NewMockNode constructs an IpfsNode for use in tests.
func NewMockNode() (*core.IpfsNode, error) {
	ctx := context.Background()

	// Generate Identity
	ident, err := testutil.RandIdentity()
	if err != nil {
		return nil, err
	}
	p := ident.ID()

	c := config.Config{
		Identity: config.Identity{
			PeerID: p.String(),
		},
	}

	nd, err := core.Offline(&repo.Mock{
		C: c,
		D: ds2.CloserWrap(syncds.MutexWrap(datastore.NewMapDatastore())),
	})(ctx)
	if err != nil {
		return nil, err
	}

	nd.PrivateKey = ident.PrivateKey()
	nd.Peerstore = peer.NewPeerstore()
	nd.Peerstore.AddPrivKey(p, ident.PrivateKey())
	nd.Peerstore.AddPubKey(p, ident.PublicKey())
	nd.Identity = p

	nd.PeerHost, err = mocknet.New(nd.Context()).AddPeer(ident.PrivateKey(), ident.Address()) // effectively offline
	if err != nil {
		return nil, err
	}

	// Routing
	nd.Routing = offrt.NewOfflineRouter(nd.Repo.Datastore(), nd.PrivateKey)

	// Bitswap
	bstore := blockstore.NewBlockstore(nd.Repo.Datastore())
	bserv, err := blockservice.New(bstore, offline.Exchange(bstore))
	if err != nil {
		return nil, err
	}

	nd.DAG = mdag.NewDAGService(bserv)

	nd.Pinning = pin.NewPinner(nd.Repo.Datastore(), nd.DAG)

	// Namespace resolver
	nd.Namesys = nsys.NewNameSystem(nd.Routing)

	// Path resolver
	nd.Resolver = &path.Resolver{DAG: nd.DAG}

	return nd, nil
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

	node, err := core.NewIPFSNode(context.Background(), core.Offline(&repo.Mock{
		D: ds2.CloserWrap(syncds.MutexWrap(datastore.NewMapDatastore())),
		C: conf,
	}))

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
