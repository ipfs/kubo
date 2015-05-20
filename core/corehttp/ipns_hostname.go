package corehttp

import (
	"net/http"
	"strings"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/ipfs/go-ipfs/core"
)

// IPNSHostnameOption rewrites an incoming request if its Host: header contains
// an IPNS name.
// The rewritten request points at the resolved name on the gateway handler.
func IPNSHostnameOption() ServeOption {
	return func(n *core.IpfsNode, mux *http.ServeMux) (*http.ServeMux, error) {
		childMux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithCancel(n.Context())
			defer cancel()

			host := strings.SplitN(r.Host, ":", 2)[0]
			if p, err := n.Namesys.Resolve(ctx, host); err == nil {
				r.URL.Path = p.String() + r.URL.Path
			}
			childMux.ServeHTTP(w, r)
		})
		return childMux, nil
	}
}
