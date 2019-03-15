package corehttp

import (
	"context"
	"net"
	"net/http"
	"strings"

	core "github.com/ipfs/go-ipfs/core"
	namesys "github.com/ipfs/go-ipfs/namesys"

	nsopts "github.com/ipfs/interface-go-ipfs-core/options/namesys"
	isd "github.com/jbenet/go-is-domain"
)

// HostnameOption rewrites an incoming request based on the Host header.
func HostnameOption() ServeOption {
	return func(n *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		childMux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithCancel(n.Context())
			defer cancel()

			// Is this one of our "known gateways"?
			if gw, ok := KnownGateways[r.Host]; ok {
				if hasPrefix(r.URL.Path, gw.PathPrefixes...) {
					childMux.ServeHTTP(w, r)
					return
				}
			} else if host, pathPrefix, ok := parseSubdomains(r.Host); ok {
				if gw, ok := KnownGateways[host]; ok && gw.SupportsSubdomains {
					// Always handle this with the gateway.
					// We don't care if it's one of the
					// valid path-prefixes.

					r.URL.Path = pathPrefix + r.URL.Path
					childMux.ServeHTTP(w, r)
					return
				}
			}

			host := strings.SplitN(r.Host, ":", 2)[0]
			if len(host) == 0 || !isd.IsDomain(host) {
				childMux.ServeHTTP(w, r)
				return
			}

			name := "/ipns/" + host
			_, err := n.Namesys.Resolve(ctx, name, nsopts.Depth(1))
			if err == nil || err == namesys.ErrResolveRecursion {
				r.URL.Path = name + r.URL.Path
			}
			childMux.ServeHTTP(w, r)
		})
		return childMux, nil
	}
}

func parseSubdomains(host string) (newHost, pathPrefix string, ok bool) {
	parts := strings.SplitN(host, ".", 3)
	if len(parts) < 3 {
		return "", "", false
	}
	switch parts[1] {
	case "ipfs", "ipns", "p2p":
		return parts[2], "/" + parts[1] + "/" + parts[0], true
	}
	return "", "", false
}

func hasPrefix(s string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}
