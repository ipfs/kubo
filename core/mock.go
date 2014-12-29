package core

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	syncds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	"github.com/jbenet/go-ipfs/blocks/blockstore"
	blockservice "github.com/jbenet/go-ipfs/blockservice"
	"github.com/jbenet/go-ipfs/exchange/offline"
	mdag "github.com/jbenet/go-ipfs/merkledag"
	nsys "github.com/jbenet/go-ipfs/namesys"
	ci "github.com/jbenet/go-ipfs/p2p/crypto"
	"github.com/jbenet/go-ipfs/p2p/net/mock"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	path "github.com/jbenet/go-ipfs/path"
	dht "github.com/jbenet/go-ipfs/routing/dht"
	ds2 "github.com/jbenet/go-ipfs/util/datastore2"
	"github.com/jbenet/go-ipfs/util/testutil"
)

// TODO this is super sketch. Deprecate and initialize one that shares code
// with the actual core constructor. Lots of fields aren't initialized.
// Additionally, the context group isn't wired up. This is as good as broken.

// NewMockNode constructs an IpfsNode for use in tests.
func NewMockNode() (*IpfsNode, error) {
	ctx := context.TODO()
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
	nd.PrivateKey = sk
	nd.Peerstore = peer.NewPeerstore()
	nd.Peerstore.AddPrivKey(p, sk)
	nd.Peerstore.AddPubKey(p, pk)
	nd.Network, err = mocknet.New(ctx).AddPeer(sk, testutil.RandLocalTCPAddress()) // effectively offline
	if err != nil {
		return nil, err
	}
	// Temp Datastore
	dstore := ds.NewMapDatastore()
	nd.Datastore = ds2.CloserWrap(syncds.MutexWrap(dstore))

	// Routing
	dht := dht.NewDHT(ctx, nd.Identity, nd.Network, nd.Datastore)
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
