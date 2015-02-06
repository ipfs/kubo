package core

import (
	"io"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"

	debugerror "github.com/jbenet/go-ipfs/util/debugerror"

	diag "github.com/jbenet/go-ipfs/diagnostics"
	ic "github.com/jbenet/go-ipfs/p2p/crypto"
	p2phost "github.com/jbenet/go-ipfs/p2p/host"
	peer "github.com/jbenet/go-ipfs/p2p/peer"

	routing "github.com/jbenet/go-ipfs/routing"

	bstore "github.com/jbenet/go-ipfs/blocks/blockstore"
	bserv "github.com/jbenet/go-ipfs/blockservice"
	exchange "github.com/jbenet/go-ipfs/exchange"
	rp "github.com/jbenet/go-ipfs/exchange/reprovide"

	merkledag "github.com/jbenet/go-ipfs/merkledag"
	namesys "github.com/jbenet/go-ipfs/namesys"
	path "github.com/jbenet/go-ipfs/path"
	pin "github.com/jbenet/go-ipfs/pin"
	repo "github.com/jbenet/go-ipfs/repo"
)

// IpfsNode is IPFS Core module. It represents an IPFS instance.
type IpfsNode struct {

	// Self
	Identity peer.ID // the local node's identity

	Repo repo.Repo

	// Local node
	Pinning    pin.Pinner // the pinning manager
	Mounts     Mounts     // current mount state, if any.
	PrivateKey ic.PrivKey // the local node's private Key

	// Services
	Peerstore  peer.Peerstore       // storage for other Peer instances
	Blockstore bstore.Blockstore    // the block store (lower level)
	Blocks     *bserv.BlockService  // the block service, get/add blocks.
	DAG        merkledag.DAGService // the merkle dag service, get/add objects.
	Resolver   *path.Resolver       // the path resolution system

	// Online
	PeerHost     p2phost.Host        // the network host (server+client)
	Bootstrapper io.Closer           // the periodic bootstrapper
	Routing      routing.IpfsRouting // the routing system. recommend ipfs-dht
	Exchange     exchange.Interface  // the block exchange + strategy (bitswap)
	Namesys      namesys.NameSystem  // the name system, resolves paths to hashes
	Diagnostics  *diag.Diagnostics   // the diagnostics service
	Reprovider   *rp.Reprovider      // the value reprovider system

	ctxgroup.ContextGroup

	mode mode
}

// NewIPFSNode is the main constructor used to create an ipfs instance
func NewIPFSNode(parent context.Context, option ConfigOption) (*IpfsNode, error) {
	ctxg := ctxgroup.WithContext(parent)
	ctx := ctxg.Context()
	success := false // flip to true after all sub-system inits succeed
	defer func() {
		if !success {
			ctxg.Close()
		}
	}()

	node, err := option(ctx)
	if err != nil {
		return nil, err
	}
	node.ContextGroup = ctxg
	ctxg.SetTeardown(node.teardown)

	// Need to make sure it's perfectly clear 1) which variables are expected
	// to be initialized at this point, and 2) which variables will be
	// initialized after this point.

	node.Blocks, err = bserv.New(node.Blockstore, node.Exchange)
	if err != nil {
		return nil, debugerror.Wrap(err)
	}
	if node.Peerstore == nil {
		node.Peerstore = peer.NewPeerstore()
	}
	node.DAG = merkledag.NewDAGService(node.Blocks)
	node.Pinning, err = pin.LoadPinner(node.Repo.Datastore(), node.DAG)
	if err != nil {
		node.Pinning = pin.NewPinner(node.Repo.Datastore(), node.DAG)
	}
	node.Resolver = &path.Resolver{DAG: node.DAG}
	success = true
	return node, nil
}
