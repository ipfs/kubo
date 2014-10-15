package core

import (
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	syncds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go/sync"
	bs "github.com/jbenet/go-ipfs/blockservice"
	ci "github.com/jbenet/go-ipfs/crypto"
	mdag "github.com/jbenet/go-ipfs/merkledag"
	nsys "github.com/jbenet/go-ipfs/namesys"
	path "github.com/jbenet/go-ipfs/path"
	"github.com/jbenet/go-ipfs/peer"
	mdht "github.com/jbenet/go-ipfs/routing/mock"
)

func NewMockNode() (*IpfsNode, error) {
	nd := new(IpfsNode)

	//Generate Identity
	nd.Identity = &peer.Peer{ID: []byte("TESTING")}
	pk, sk, err := ci.GenerateKeyPair(ci.RSA, 1024)
	if err != nil {
		return nil, err
	}
	nd.Identity.PrivKey = pk
	nd.Identity.PubKey = sk

	// Temp Datastore
	dstore := ds.NewMapDatastore()
	nd.Datastore = syncds.MutexWrap(dstore)

	// Routing
	dht := mdht.NewMockRouter(nd.Identity, nd.Datastore)
	nd.Routing = dht

	// Bitswap
	//??

	bserv, err := bs.NewBlockService(nd.Datastore, nil)
	if err != nil {
		return nil, err
	}

	nd.DAG = mdag.NewDAGService(bserv)

	// Namespace resolver
	nd.Namesys = nsys.NewNameSystem(dht)

	// Path resolver
	nd.Resolver = &path.Resolver{nd.DAG}

	return nd, nil
}
