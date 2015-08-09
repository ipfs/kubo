package core

import (
	"crypto/rand"
	"encoding/base64"
	"errors"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dsync "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	goprocessctx "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess/context"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	bstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	key "github.com/ipfs/go-ipfs/blocks/key"
	bserv "github.com/ipfs/go-ipfs/blockservice"
	offline "github.com/ipfs/go-ipfs/exchange/offline"
	dag "github.com/ipfs/go-ipfs/merkledag"
	ci "github.com/ipfs/go-ipfs/p2p/crypto"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	path "github.com/ipfs/go-ipfs/path"
	pin "github.com/ipfs/go-ipfs/pin"
	repo "github.com/ipfs/go-ipfs/repo"
	cfg "github.com/ipfs/go-ipfs/repo/config"
)

var ErrAlreadyBuilt = errors.New("this builder has already been used")

// NodeBuilder is an object used to generate an IpfsNode
type NodeBuilder struct {
	online   bool
	routing  RoutingOption
	peerhost HostOption
	repo     repo.Repo
	built    bool
	nilrepo  bool
}

func NewNodeBuilder() *NodeBuilder {
	return &NodeBuilder{
		online:   false,
		routing:  DHTOption,
		peerhost: DefaultHostOption,
	}
}

func defaultRepo(dstore ds.ThreadSafeDatastore) (repo.Repo, error) {
	c := cfg.Config{}
	priv, pub, err := ci.GenerateKeyPairWithReader(ci.RSA, 1024, rand.Reader)
	if err != nil {
		return nil, err
	}

	data, err := pub.Hash()
	if err != nil {
		return nil, err
	}

	privkeyb, err := priv.Bytes()
	if err != nil {
		return nil, err
	}

	c.Bootstrap = cfg.DefaultBootstrapAddresses
	c.Addresses.Swarm = []string{"/ip4/0.0.0.0/tcp/4001"}
	c.Identity.PeerID = key.Key(data).B58String()
	c.Identity.PrivKey = base64.StdEncoding.EncodeToString(privkeyb)

	return &repo.Mock{
		D: dstore,
		C: c,
	}, nil
}

func (nb *NodeBuilder) Online() *NodeBuilder {
	nb.online = true
	return nb
}

func (nb *NodeBuilder) Offline() *NodeBuilder {
	nb.online = false
	return nb
}

func (nb *NodeBuilder) SetRouting(ro RoutingOption) *NodeBuilder {
	nb.routing = ro
	return nb
}

func (nb *NodeBuilder) SetHost(ho HostOption) *NodeBuilder {
	nb.peerhost = ho
	return nb
}

func (nb *NodeBuilder) SetRepo(r repo.Repo) *NodeBuilder {
	nb.repo = r
	return nb
}

func (nb *NodeBuilder) NilRepo() *NodeBuilder {
	nb.nilrepo = true
	return nb
}

func (nb *NodeBuilder) Build(ctx context.Context) (*IpfsNode, error) {
	if nb.built {
		return nil, ErrAlreadyBuilt
	}
	nb.built = true
	if nb.repo == nil {
		var d ds.Datastore
		d = ds.NewMapDatastore()
		if nb.nilrepo {
			d = ds.NewNullDatastore()
		}
		r, err := defaultRepo(dsync.MutexWrap(d))
		if err != nil {
			return nil, err
		}
		nb.repo = r
	}

	n := &IpfsNode{
		mode:      offlineMode,
		Repo:      nb.repo,
		ctx:       ctx,
		Peerstore: peer.NewPeerstore(),
	}
	if nb.online {
		n.mode = onlineMode
	}

	// TODO: this is a weird circular-ish dependency, rework it
	n.proc = goprocessctx.WithContextAndTeardown(ctx, n.teardown)

	success := false
	defer func() {
		if !success {
			n.teardown()
		}
	}()

	// setup local peer ID (private key is loaded in online setup)
	if err := n.loadID(); err != nil {
		return nil, err
	}

	var err error
	n.Blockstore, err = bstore.WriteCached(bstore.NewBlockstore(n.Repo.Datastore()), kSizeBlockstoreWriteCache)
	if err != nil {
		return nil, err
	}

	if nb.online {
		do := setupDiscoveryOption(n.Repo.Config().Discovery)
		if err := n.startOnlineServices(ctx, nb.routing, nb.peerhost, do); err != nil {
			return nil, err
		}
	} else {
		n.Exchange = offline.Exchange(n.Blockstore)
	}

	n.Blocks, err = bserv.New(n.Blockstore, n.Exchange)
	if err != nil {
		return nil, err
	}

	n.DAG = dag.NewDAGService(n.Blocks)
	n.Pinning, err = pin.LoadPinner(n.Repo.Datastore(), n.DAG)
	if err != nil {
		// TODO: we should move towards only running 'NewPinner' explicity on
		// node init instead of implicitly here as a result of the pinner keys
		// not being found in the datastore.
		// this is kinda sketchy and could cause data loss
		n.Pinning = pin.NewPinner(n.Repo.Datastore(), n.DAG)
	}
	n.Resolver = &path.Resolver{DAG: n.DAG}

	success = true
	return n, nil
}
