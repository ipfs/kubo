package core

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	syncds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"

	"github.com/jbenet/go-ipfs/exchange/offline"
	nsys "github.com/jbenet/go-ipfs/namesys"
	mocknet "github.com/jbenet/go-ipfs/p2p/net/mock"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	path "github.com/jbenet/go-ipfs/path"
	"github.com/jbenet/go-ipfs/repo"
	mockrouting "github.com/jbenet/go-ipfs/routing/mock"
	blockservice "github.com/jbenet/go-ipfs/struct/blocks/blockservice"
	"github.com/jbenet/go-ipfs/struct/blocks/blockstore"
	mdag "github.com/jbenet/go-ipfs/struct/merkledag"
	ds2 "github.com/jbenet/go-ipfs/thirdparty/datastore2"
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
	nd.Routing = mockrouting.NewServer().Client(ident)

	// Bitswap
	bstore := blockstore.NewBlockstore(nd.Repo.Datastore())
	bserv, err := blockservice.New(bstore, offline.Exchange(bstore))
	if err != nil {
		return nil, err
	}

	nd.DAG = mdag.NewDAGService(bserv)

	// Namespace resolver
	nd.Namesys = nsys.NewNameSystem(nd.Routing)

	// Path resolver
	nd.Resolver = &path.Resolver{DAG: nd.DAG}

	return nd, nil
}
