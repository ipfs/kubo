package namesys

import (
	"errors"

	mdag "github.com/jbenet/go-ipfs/merkledag"
	"github.com/jbenet/go-ipfs/routing"
)

var ErrCouldntResolve = errors.New("could not resolve name.")

type masterResolver struct {
	res []Resolver
}

func NewMasterResolver(r routing.IpfsRouting, dag *mdag.DAGService) Resolver {
	mr := new(masterResolver)
	mr.res = []Resolver{
		new(DNSResolver),
		new(ProquintResolver),
		NewRoutingResolver(r, dag),
	}
	return mr
}

func (mr *masterResolver) Resolve(name string) (string, error) {
	for _, r := range mr.res {
		if r.Matches(name) {
			return r.Resolve(name)
		}
	}
	return "", ErrCouldntResolve
}

func (mr *masterResolver) Matches(name string) bool {
	for _, r := range mr.res {
		if r.Matches(name) {
			return true
		}
	}
	return false
}
