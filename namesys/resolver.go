package namesys

import (
	"errors"

	mdag "github.com/jbenet/go-ipfs/merkledag"
	"github.com/jbenet/go-ipfs/routing"
)

var ErrCouldntResolve = errors.New("could not resolve name.")

type MasterResolver struct {
	res []Resolver
}

func NewMasterResolver(r routing.IpfsRouting, dag *mdag.DAGService) *MasterResolver {
	mr := new(MasterResolver)
	mr.res = []Resolver{
		new(DNSResolver),
		new(ProquintResolver),
		NewRoutingResolver(r, dag),
	}
	return mr
}

func (mr *MasterResolver) Resolve(name string) (string, error) {
	for _, r := range mr.res {
		if r.Matches(name) {
			return r.Resolve(name)
		}
	}
	return "", ErrCouldntResolve
}

func (mr *MasterResolver) Matches(name string) bool {
	for _, r := range mr.res {
		if r.Matches(name) {
			return true
		}
	}
	return false
}
