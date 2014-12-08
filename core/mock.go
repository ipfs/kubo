package core

import (
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	syncds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	"github.com/jbenet/go-ipfs/blocks/blockstore"
	blockservice "github.com/jbenet/go-ipfs/blockservice"
	ci "github.com/jbenet/go-ipfs/crypto"
	"github.com/jbenet/go-ipfs/exchange/offline"
	mdag "github.com/jbenet/go-ipfs/merkledag"
	nsys "github.com/jbenet/go-ipfs/namesys"
	path "github.com/jbenet/go-ipfs/path"
	peer "github.com/jbenet/go-ipfs/peer"
	mdht "github.com/jbenet/go-ipfs/routing/mock"
	"github.com/jbenet/go-ipfs/util"
)

// NewMockNode constructs an IpfsNode for use in tests.
func NewMockNode() (*IpfsNode, error) {
	nd := new(IpfsNode)

	// Generate Identity
	sk, pk, err := ci.GenerateKeyPair(ci.RSA, 1024)
	if err != nil {
		return nil, err
	}

	nd.Peerstore = peer.NewPeerstore()

	p, err := nd.Peerstore.WithKeyPair(sk, pk)
	if err != nil {
		return nil, err
	}

	nd.Identity, err = nd.Peerstore.Add(p)
	if err != nil {
		return nil, err
	}

	// Temp Datastore
	dstore := ds.NewMapDatastore()
	nd.Datastore = util.CloserWrap(syncds.MutexWrap(dstore))

	// Routing
	dht := mdht.NewMockRouter(nd.Identity, nd.Datastore)
	nd.Routing = dht

	// Bitswap
	bstore := blockstore.NewBlockstore(nd.Datastore)
	bserv, err := blockservice.New(bstore, offline.Exchange(bstore))
	if err != nil {
		return nil, err
	}

	nd.DAG = mdag.NewDAGService(bserv)

	// Namespace resolver
	nd.Namesys = nsys.NewNameSystem(dht)

	// Path resolver
	nd.Resolver = &path.Resolver{DAG: nd.DAG}

	return nd, nil
}
