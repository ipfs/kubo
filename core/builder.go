package core

import (
	"crypto/rand"
	"encoding/base64"
	"errors"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dsync "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	key "github.com/ipfs/go-ipfs/blocks/key"
	ci "github.com/ipfs/go-ipfs/p2p/crypto"
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

func defaultRepo(dstore repo.Datastore) (repo.Repo, error) {
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
	conf := standardWithRouting(nb.repo, nb.online, nb.routing, nb.peerhost)
	return NewIPFSNode(ctx, conf)
}
