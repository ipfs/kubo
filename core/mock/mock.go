package coremock

import (
	ctxgroup "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	syncds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/ipfs/go-ipfs/blocks/blockstore"
	blockservice "github.com/ipfs/go-ipfs/blockservice"
	core "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/exchange/offline"
	mdag "github.com/ipfs/go-ipfs/merkledag"
	nsys "github.com/ipfs/go-ipfs/namesys"
	mocknet "github.com/ipfs/go-ipfs/p2p/net/mock"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	path "github.com/ipfs/go-ipfs/path"
	pin "github.com/ipfs/go-ipfs/pin"
	"github.com/ipfs/go-ipfs/repo"
	offrt "github.com/ipfs/go-ipfs/routing/offline"
	ds2 "github.com/ipfs/go-ipfs/util/datastore2"
	testutil "github.com/ipfs/go-ipfs/util/testutil"
)

// TODO this is super sketch. Deprecate and initialize one that shares code
// with the actual core constructor. Lots of fields aren't initialized.
// Additionally, the context group isn't wired up. This is as good as broken.

// NewMockNode constructs an IpfsNode for use in tests.
func NewMockNode() (*core.IpfsNode, error) {
	ctx := context.TODO()
	nd := new(core.IpfsNode)

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
