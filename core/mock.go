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
	ds2 "github.com/jbenet/go-ipfs/util/datastore2"
)

// NewMockNode constructs an IpfsNode for use in tests.
func NewMockNode() (*IpfsNode, error) {
	nd := new(IpfsNode)

	// Generate Identity
	sk, pk, err := ci.GenerateKeyPair(ci.RSA, 1024)
	if err != nil {
		return nil, err
	}

	p, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return nil, err
	}

	nd.Identity = p
	nd.Peerstore = peer.NewPeerstore()
	nd.Peerstore.AddPrivKey(p, sk)
	nd.Peerstore.AddPubKey(p, pk)

	// Temp Datastore
	dstore := ds.NewMapDatastore()
	nd.Datastore = ds2.CloserWrap(syncds.MutexWrap(dstore))

	// Routing
	dht := mdht.NewServer().ClientWithDatastore(peer.PeerInfo{ID: p}, nd.Datastore)
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
