package testutil

import (
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	syncds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go/sync"
	bs "github.com/jbenet/go-ipfs/blockservice"
	core "github.com/jbenet/go-ipfs/core"
	ci "github.com/jbenet/go-ipfs/crypto"
	mdag "github.com/jbenet/go-ipfs/merkledag"
	nsys "github.com/jbenet/go-ipfs/namesys"
	"github.com/jbenet/go-ipfs/peer"
	mdht "github.com/jbenet/go-ipfs/routing/mock"
)

var _ = core.IpfsNode{}

func NewMockNode() (*core.IpfsNode, error) {
	nd := new(core.IpfsNode)

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

	// Bitswap
	//??

	bserv, err := bs.NewBlockService(nd.Datastore, nil)
	if err != nil {
		return nil, err
	}

	dserv := &mdag.DAGService{bserv}

	// Namespace resolver
	nd.Namesys = nsys.NewMasterResolver(dht, dserv)
	return nd, nil
}
