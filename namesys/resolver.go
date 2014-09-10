package namesys

import "strings"

type MasterResolver struct {
	dns     *DNSResolver
	routing *RoutingResolver
	pro     *ProquintResolver
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
