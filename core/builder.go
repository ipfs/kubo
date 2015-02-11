package core

import (
	"errors"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dsync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"

	repo "github.com/jbenet/go-ipfs/repo"
)

var ErrAlreadyBuilt = errors.New("this builder has already been used")

// NodeBuilder is an object used to generate an IpfsNode
type NodeBuilder struct {
	online   bool
	routing  RoutingOption
	peerhost HostOption
	repo     repo.Repo
	built    bool
}

func NewNodeBuilder() *NodeBuilder {
	return &NodeBuilder{
		online:   false,
		routing:  DHTOption,
		peerhost: DefaultHostOption,
	}
}

func defaultRepo() repo.Repo {
	return &repo.Mock{
		D: dsync.MutexWrap(ds.NewMapDatastore()),
	}
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

func (nb *NodeBuilder) Build(ctx context.Context) (*IpfsNode, error) {
	if nb.built {
		return nil, ErrAlreadyBuilt
	}
	nb.built = true
	if nb.repo == nil {
		nb.repo = defaultRepo()
	}
	conf := standardWithRouting(nb.repo, nb.online, nb.routing, nb.peerhost)
	return NewIPFSNode(ctx, conf)
}
