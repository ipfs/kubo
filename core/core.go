package core

import (
	"errors"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"

	bserv "github.com/jbenet/go-ipfs/blockservice"
	config "github.com/jbenet/go-ipfs/config"
	ic "github.com/jbenet/go-ipfs/crypto"
	diag "github.com/jbenet/go-ipfs/diagnostics"
	exchange "github.com/jbenet/go-ipfs/exchange"
	mount "github.com/jbenet/go-ipfs/fuse/mount"
	merkledag "github.com/jbenet/go-ipfs/merkledag"
	namesys "github.com/jbenet/go-ipfs/namesys"
	inet "github.com/jbenet/go-ipfs/net"
	path "github.com/jbenet/go-ipfs/path"
	peer "github.com/jbenet/go-ipfs/peer"
	pin "github.com/jbenet/go-ipfs/pin"
	repo "github.com/jbenet/go-ipfs/repo"
	routing "github.com/jbenet/go-ipfs/routing"
	ds2 "github.com/jbenet/go-ipfs/util/datastore2"
	eventlog "github.com/jbenet/go-ipfs/util/eventlog"
)

const IpnsValidatorTag = "ipns"
const kSizeBlockstoreWriteCache = 100

var log = eventlog.Logger("core")

// IpfsNode is IPFS Core module. It represents an IPFS instance.
type IpfsNode struct {

	// Self
	Config     *config.Config // the node's configuration
	Identity   peer.ID        // the local node's identity
	PrivateKey ic.PrivKey     // the local node's private Key
	onlineMode bool           // alternatively, offline

	// Local node
	Datastore ds2.ThreadSafeDatastoreCloser // the local datastore
	Pinning   pin.Pinner                    // the pinning manager
	Mounts    Mounts                        // current mount state, if any. // TODO mounts are never assigned in initializer

	// Services
	Peerstore   peer.Peerstore       // storage for other Peer instances
	Network     inet.Network         // the network message stream
	Routing     routing.IpfsRouting  // the routing system. recommend ipfs-dht
	Exchange    exchange.Interface   // the block exchange + strategy (bitswap)
	Blocks      *bserv.BlockService  // the block service, get/add blocks.
	DAG         merkledag.DAGService // the merkle dag service, get/add objects.
	Resolver    *path.Resolver       // the path resolution system
	Namesys     namesys.NameSystem   // the name system, resolves paths to hashes
	Diagnostics *diag.Diagnostics    // the diagnostics service

	ctxgroup.ContextGroup
}

// Mounts defines what the node's mount state is. This should
// perhaps be moved to the daemon or mount. It's here because
// it needs to be accessible across daemon requests.
type Mounts struct {
	Ipfs mount.Mount
	Ipns mount.Mount
}

// NewIpfsNode constructs a new IpfsNode based on the given config.
// TODO remove this method once tests pass and both cases are handled.
func NewIpfsNode(parent context.Context, cfg *config.Config, online bool) (n *IpfsNode, err error) {
	if online {
		return NewIpfsNodeWithRepo(parent, repo.Online(cfg))
	}
	return nil, errors.New("TODO offline")
}

func NewIpfsNodeWithRepo(parent context.Context, repoConf repo.RepoConfig) (n *IpfsNode, err error) {
	ctxg := ctxgroup.WithContext(parent)
	ctx := ctxg.Context()
	success := false // flip to true after all sub-system inits succeed
	defer func() {
		if !success {
			ctxg.Close()
		}
	}()
	// TODO handle offline
	r, err := repoConf(ctx)
	if err != nil {
		return nil, err
	}
	blockservice, err := bserv.New(r.Blockstore(), r.Exchange())
	if err != nil {
		return nil, err
	}
	pinner, err := pin.LoadPinner(n.Datastore, n.DAG)
	if err != nil {
		pinner = pin.NewPinner(n.Datastore, n.DAG)
	}
	dag := merkledag.NewDAGService(blockservice)

	// create the node as late as possible. Don't assign to it using `n.Field =
	// foo`. Assign into the struct literal. This makes it possible to
	// cross-reference the struct declaration, ensuring all fields are either
	// a) being assigned to or b) deliberately omitted (with an explanatory
	// note).
	// NB: Mounts never assigned. This is intentional.
	// NB: Config never assigned. This is intentional. TODO remove config?
	n = &IpfsNode{
		Blocks:       blockservice,
		ContextGroup: ctxgroup.WithContext(ctx),
		DAG:          dag,
		Datastore:    r.Datastore(),
		Diagnostics:  diag.NewDiagnostics(r.ID(), r.Network()),
		Exchange:     r.Exchange(),
		Identity:     r.ID(),
		Namesys:      namesys.NewNameSystem(r.Routing()),
		Network:      r.Network(),
		Peerstore:    r.Peerstore(),
		Pinning:      pinner,
		PrivateKey:   r.PrivateKey(),
		Resolver:     &path.Resolver{DAG: dag},
		Routing:      r.Routing(),
		onlineMode:   r.OnlineMode(),
	}
	ctx = n.ContextGroup.Context()
	n.AddChildGroup(n.Network.CtxGroup()) // NB don't want to do this is network is nil (in offline mode)
	n.AddChildGroup(r.Routing())
	n.ContextGroup.SetTeardown(n.teardown)
	success = true
	return n, nil
}

func (n *IpfsNode) teardown() error {
	if err := n.Datastore.Close(); err != nil {
		return err
	}
	return nil
}

func (n *IpfsNode) OnlineMode() bool {
	return n.onlineMode
}
