package filter

import (
	"net"
	"strings"

	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
)

type Filters struct {
	filters []*net.IPNet
}

func (fs *Filters) AddDialFilter(f *net.IPNet) {
	fs.filters = append(fs.filters, f)
}

func (f *Filters) AddrBlocked(a ma.Multiaddr) bool {
	_, addr, err := manet.DialArgs(a)
	if err != nil {
		// if we cant parse it, its probably not blocked
		return false
	}

	ipstr := strings.Split(addr, ":")[0]
	ip := net.ParseIP(ipstr)
	for _, ft := range f.filters {
		if ft.Contains(ip) {
			return true
		}
	}
	return false
}
