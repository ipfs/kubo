package core

import (
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	syncds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/jbenet/go-ipfs/blocks/blockstore"
	blockservice "github.com/jbenet/go-ipfs/blockservice"
	"github.com/jbenet/go-ipfs/exchange/offline"
	mdag "github.com/jbenet/go-ipfs/merkledag"
	nsys "github.com/jbenet/go-ipfs/namesys"
	mocknet "github.com/jbenet/go-ipfs/p2p/net/mock"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	path "github.com/jbenet/go-ipfs/path"
	pin "github.com/jbenet/go-ipfs/pin"
	"github.com/jbenet/go-ipfs/repo"
	offrt "github.com/jbenet/go-ipfs/routing/offline"
	ds2 "github.com/jbenet/go-ipfs/util/datastore2"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

// TODO this is super sketch. Deprecate and initialize one that shares code
// with the actual core constructor. Lots of fields aren't initialized.
// Additionally, the context group isn't wired up. This is as good as broken.

// NewMockNode constructs an IpfsNode for use in tests.
func NewMockNode() (*IpfsNode, error) {
	ctx := context.TODO()
	nd := new(IpfsNode)

	// Generate Identity
	ident, err := testutil.RandIdentity()
	if err != nil {
		return nil, err
	}

	p := ident.ID()
	nd.Identity = p
	nd.PrivateKey = ident.PrivateKey()
	nd.Peerstore = peer.NewPeerstore()
	nd.Peerstore.AddPrivKey(p, ident.PrivateKey())
	nd.Peerstore.AddPubKey(p, ident.PublicKey())
	nd.ContextGroup = ctxgroup.WithContext(ctx)

	nd.PeerHost, err = mocknet.New(ctx).AddPeer(ident.PrivateKey(), ident.Address()) // effectively offline
	if err != nil {
		return nil, err
	}

	// Temp Datastore
	nd.Repo = &repo.Mock{
		// TODO C: conf,
		D: ds2.CloserWrap(syncds.MutexWrap(datastore.NewMapDatastore())),
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
