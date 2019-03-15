package corehttp

import (
	"context"
	"fmt"
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

			// Unfortunately, many (well, ipfs.io) gateways use
			// DNSLink so if we blindly rewrite with DNSLink, we'll
			// break /ipfs links.
			//
			// We fix this by maintaining a list of known gateways
			// and the paths that they serve "gateway" content on.
			// That way, we can use DNSLink for everything else.
			//
			// TODO: We wouldn't need _any_ of this if we
			// supported transparent symlink resolution on
			// the gateway. If we had that, such gateways could add
			// symlinks to `/ipns`, `/ipfs`, `/api`, etc. to their
			// home directories. That way, `/ipfs/QmA/ipfs/QmB`
			// would "just work". Should we try this?

			// Is this one of our "known gateways"?
			if gw, ok := KnownGateways[r.Host]; ok {
				// This is a known gateway but we're not using
				// the subdomain feature.

				// Does this gateway _handle_ this path?
				if hasPrefix(r.URL.Path, gw.PathPrefixes...) {
					// It does.

					// Does this gateway use subdomains?
					if gw.UseSubdomains {
						// Yes, redirect if applicable (pretty much everything except `/api`).
						if newURL, ok := toSubdomainURL(r.Host, r.URL.Path); ok {
							http.Redirect(
								w, r, newURL, http.StatusMovedPermanently,
							)
							return
						}
					}
					childMux.ServeHTTP(w, r)
					return
				}
			} else if host, pathPrefix, ok := parseSubdomains(r.Host); ok {
				// Looks like we're using subdomains.

				// Again, is this a known gateway that supports subdomains?
				if gw, ok := KnownGateways[host]; ok && gw.UseSubdomains {

					// Yes, serve the request (and rewrite the path to not use subdomains).
					r.URL.Path = pathPrefix + r.URL.Path
					childMux.ServeHTTP(w, r)
					return
				}
			}

			// We don't have a known gateway. Fallback on dnslink.
			host := strings.SplitN(r.Host, ":", 2)[0]
			if len(host) > 0 && isd.IsDomain(host) {
				name := "/ipns/" + host
				_, err := n.Namesys.Resolve(ctx, name, nsopts.Depth(1))
				if err == nil || err == namesys.ErrResolveRecursion {
					// The domain supports dnslink, rewrite.
					r.URL.Path = name + r.URL.Path
				}
			} // else, just treat it as a gateway, I guess.

			childMux.ServeHTTP(w, r)
		})
		return childMux, nil
	}
}

func isSubdomainNamespace(ns string) bool {
	switch ns {
	case "ipfs", "ipns", "p2p", "ipld":
		return true
	default:
		return false
	}
}

// Parses a subdomain-based URL and returns it's components
func parseSubdomains(host string) (newHost, pathPrefix string, ok bool) {
	parts := strings.SplitN(host, ".", 3)
	if len(parts) < 3 || !isSubdomainNamespace(parts[1]) {
		return "", "", false
	}
	return parts[2], "/" + parts[1] + "/" + parts[0], true
}

// Converts a host/path to a subdomain-based URL, if applicable.
func toSubdomainURL(host, path string) (url string, ok bool) {
	parts := strings.SplitN(path, "/", 4)

	var ns, object, rest string
	switch len(parts) {
	case 4:
		rest = parts[3]
		fallthrough
	case 3:
		ns = parts[1]
		object = parts[2]
	default:
		return "", false
	}

	if !isSubdomainNamespace(ns) {
		return "", false
	}

	return fmt.Sprintf(
		"http://%s.%s.%s/%s",
		object,
		ns,
		host,
		rest,
	), true
}

func hasPrefix(s string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}
