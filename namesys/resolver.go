package namesys

import (
	"strings"

	mdag "github.com/jbenet/go-ipfs/merkledag"
	"github.com/jbenet/go-ipfs/routing"
)

type MasterResolver struct {
	dns     *DNSResolver
	routing *RoutingResolver
	pro     *ProquintResolver
}

func NewMasterResolver(r routing.IpfsRouting, dag *mdag.DAGService) *MasterResolver {
	mr := new(MasterResolver)
	mr.dns = new(DNSResolver)
	mr.pro = new(ProquintResolver)
	mr.routing = NewRoutingResolver(r, dag)
	return mr
}

func (mr *MasterResolver) Resolve(name string) (string, error) {
	if strings.Contains(name, ".") {
		return mr.dns.Resolve(name)
	}

	if strings.Contains(name, "-") {
		return mr.pro.Resolve(name)
	}

	return mr.routing.Resolve(name)
}
