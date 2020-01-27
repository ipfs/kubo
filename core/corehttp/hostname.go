package corehttp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"

	cid "github.com/ipfs/go-cid"
	core "github.com/ipfs/go-ipfs/core"
	namesys "github.com/ipfs/go-ipfs/namesys"

	config "github.com/ipfs/go-ipfs-config"
	nsopts "github.com/ipfs/interface-go-ipfs-core/options/namesys"
	isd "github.com/jbenet/go-is-domain"
)

var defaultGatewaySpec = config.GatewaySpec{
	PathPrefixes:  []string{ipfsPathPrefix, ipnsPathPrefix, "/api/"},
	UseSubdomains: false,
}

var defaultKnownGateways = map[string]config.GatewaySpec{
	"localhost": config.GatewaySpec{
		PathPrefixes:  []string{ipfsPathPrefix, ipnsPathPrefix},
		UseSubdomains: true,
	},
	"ipfs.io":         defaultGatewaySpec,
	"gateway.ipfs.io": defaultGatewaySpec,
	"dweb.link": config.GatewaySpec{
		PathPrefixes:  []string{ipfsPathPrefix, ipnsPathPrefix},
		UseSubdomains: true,
	},
}

// Find content identifier, protocol, and remaining hostname
// of a subdomain gateway (eg. *.ipfs.foo.bar.co.uk)
var subdomainGatewayRegex = regexp.MustCompile("^(.+).(ipfs|ipns|ipld|p2p).([^:/]+)")

// HostnameOption rewrites an incoming request based on the Host header.
func HostnameOption() ServeOption {
	return func(n *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		childMux := http.NewServeMux()

		cfg, err := n.Repo.Config()
		if err != nil {
			return nil, err
		}
		knownGateways := make(
			map[string]config.GatewaySpec,
			len(defaultKnownGateways)+len(cfg.Gateway.PublicGateways),
		)
		for host, gw := range defaultKnownGateways {
			knownGateways[host] = gw
		}
		for host, gw := range cfg.Gateway.PublicGateways {
			if gw == nil {
				// Allows the user to remove gateways but _also_
				// allows us to continuously update the list.
				delete(knownGateways, host)
			} else {
				knownGateways[host] = *gw
			}
		}

		// Return matching GatewaySpec with gracefull fallback to version without port
		isKnownGateway := func(host string) (gw config.GatewaySpec, ok bool) {
			// Try host+(optional)port (value from Host header as-is)
			if gw, ok := knownGateways[host]; ok {
				return gw, ok
			}
			// Fallback to hostname without port
			gw, ok = knownGateways[stripPort(host)]
			return gw, ok
		}

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

			// TODO: verify this sidenote: in proxy mode, r.URL.Host is the host of the target
			// server and r.Host is the host of the proxy server itself.

			// HTTP Host check: is this one of our path-based "known gateways"?
			if gw, ok := isKnownGateway(r.Host); ok {
				// This is a known gateway but we're not using
				// the subdomain feature.

				// Does this gateway _handle_ this path?
				if hasPrefix(r.URL.Path, gw.PathPrefixes...) {
					// It does.

					// Should this gateway use subdomains instead of paths?
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
			}

			// HTTP Host check: is this one of our subdomain-based "known gateways"?
			if host, pathPrefix, ok := parseSubdomains(r.Host); ok {
				// Looks like we're using subdomains.

				// Again, is this a known gateway that supports subdomains?
				if gw, ok := isKnownGateway(host); ok && gw.UseSubdomains {

					// Yes, serve the request (and rewrite the path to not use subdomains).
					r.URL.Path = pathPrefix + r.URL.Path
					childMux.ServeHTTP(w, r)
					return
				}
			}
			// We don't have a known gateway. Fallback on DNSLink lookup

			// HTTP Host check: does Host header include a fully qualified domain name (FQDN)?
			fqdn := stripPort(r.Host)
			if len(fqdn) > 0 && isd.IsDomain(fqdn) {
				gw, ok := isKnownGateway(fqdn)
				// Confirm DNSLink was not disabled for the fqdn or globally
				enabled := (ok && !gw.NoDNSLink) || !cfg.Gateway.GatewaySpec.NoDNSLink
				if enabled {
					name := "/ipns/" + fqdn
					_, err := n.Namesys.Resolve(ctx, name, nsopts.Depth(1))
					if err == nil || err == namesys.ErrResolveRecursion {
						// Check if this gateway has any PathPrefixes mounted
						if ok && hasPrefix(r.URL.Path, gw.PathPrefixes...) {
							// Yes: PathPrefixes should take priority over DNSLink
							childMux.ServeHTTP(w, r)
							return
						}
						// The domain supports DNSLink, rewrite.
						r.URL.Path = name + r.URL.Path
					}
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
	parts := subdomainGatewayRegex.FindStringSubmatch(host)
	if len(parts) < 4 || !isSubdomainNamespace(parts[2]) {
		return "", "", false
	}
	return parts[3], "/" + parts[2] + "/" + parts[1], true
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

	rootCid, err := cid.Decode(object)
	if err == nil {
		// if object turns out to be a valid CID,
		// ensure text representation used in subdomain is CIDv1 in Base32
		// https://github.com/ipfs/in-web-browsers/issues/89
		object = cid.NewCidV1(rootCid.Type(), rootCid.Hash()).String()
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

func stripPort(host string) string {
	return strings.SplitN(host, ":", 2)[0]
}
