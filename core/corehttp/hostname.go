package corehttp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	cid "github.com/ipfs/go-cid"
	core "github.com/ipfs/go-ipfs/core"
	coreapi "github.com/ipfs/go-ipfs/core/coreapi"
	namesys "github.com/ipfs/go-ipfs/namesys"
	isd "github.com/jbenet/go-is-domain"
	"github.com/libp2p/go-libp2p-core/peer"
	mbase "github.com/multiformats/go-multibase"

	config "github.com/ipfs/go-ipfs-config"
	iface "github.com/ipfs/interface-go-ipfs-core"
	options "github.com/ipfs/interface-go-ipfs-core/options"
	nsopts "github.com/ipfs/interface-go-ipfs-core/options/namesys"
)

var defaultPaths = []string{"/ipfs/", "/ipns/", "/api/", "/p2p/", "/version"}

var pathGatewaySpec = config.GatewaySpec{
	Paths:         defaultPaths,
	UseSubdomains: false,
}

var subdomainGatewaySpec = config.GatewaySpec{
	Paths:         defaultPaths,
	UseSubdomains: true,
}

var defaultKnownGateways = map[string]config.GatewaySpec{
	"localhost":       subdomainGatewaySpec,
	"ipfs.io":         pathGatewaySpec,
	"gateway.ipfs.io": pathGatewaySpec,
	"dweb.link":       subdomainGatewaySpec,
}

// HostnameOption rewrites an incoming request based on the Host header.
func HostnameOption() ServeOption {
	return func(n *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		childMux := http.NewServeMux()

		coreApi, err := coreapi.NewCoreAPI(n)
		if err != nil {
			return nil, err
		}

		cfg, err := n.Repo.Config()
		if err != nil {
			return nil, err
		}
		knownGateways := make(
			map[string]config.GatewaySpec,
			len(defaultKnownGateways)+len(cfg.Gateway.PublicGateways),
		)
		for hostname, gw := range defaultKnownGateways {
			knownGateways[hostname] = gw
		}
		for hostname, gw := range cfg.Gateway.PublicGateways {
			if gw == nil {
				// Allows the user to remove gateways but _also_
				// allows us to continuously update the list.
				delete(knownGateways, hostname)
			} else {
				knownGateways[hostname] = *gw
			}
		}

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Unfortunately, many (well, ipfs.io) gateways use
			// DNSLink so if we blindly rewrite with DNSLink, we'll
			// break /ipfs links.
			//
			// We fix this by maintaining a list of known gateways
			// and the paths that they serve "gateway" content on.
			// That way, we can use DNSLink for everything else.

			// HTTP Host & Path check: is this one of our  "known gateways"?
			if gw, ok := isKnownHostname(r.Host, knownGateways); ok {
				// This is a known gateway but request is not using
				// the subdomain feature.

				// Does this gateway _handle_ this path?
				if hasPrefix(r.URL.Path, gw.Paths...) {
					// It does.

					// Should this gateway use subdomains instead of paths?
					if gw.UseSubdomains {
						// Yes, redirect if applicable
						// Example: dweb.link/ipfs/{cid} → {cid}.ipfs.dweb.link
						if newURL, ok := toSubdomainURL(r.Host, r.URL.Path, r); ok {
							// Just to be sure single Origin can't be abused in
							// web browsers that ignored the redirect for some
							// reason, Clear-Site-Data header clears browsing
							// data (cookies, storage etc) associated with
							// hostname's root Origin
							// Note: we can't use "*" due to bug in Chromium:
							// https://bugs.chromium.org/p/chromium/issues/detail?id=898503
							w.Header().Set("Clear-Site-Data", "\"cookies\", \"storage\"")

							// Set "Location" header with redirect destination.
							// It is ignored by curl in default mode, but will
							// be respected by user agents that follow
							// redirects by default, namely web browsers
							w.Header().Set("Location", newURL)

							// Note: we continue regular gateway processing:
							// HTTP Status Code http.StatusMovedPermanently
							// will be set later, in statusResponseWriter
						}
					}

					// Not a subdomain resource, continue with path processing
					// Example: 127.0.0.1:8080/ipfs/{CID}, ipfs.io/ipfs/{CID} etc
					childMux.ServeHTTP(w, r)
					return
				}
				// Not a whitelisted path

				// Try DNSLink, if it was not explicitly disabled for the hostname
				if !gw.NoDNSLink && isDNSLinkRequest(n.Context(), coreApi, r) {
					// rewrite path and handle as DNSLink
					r.URL.Path = "/ipns/" + stripPort(r.Host) + r.URL.Path
					childMux.ServeHTTP(w, r)
					return
				}

				// If not, resource does not exist on the hostname, return 404
				http.NotFound(w, r)
				return
			}

			// HTTP Host check: is this one of our subdomain-based "known gateways"?
			// Example: {cid}.ipfs.localhost, {cid}.ipfs.dweb.link
			if gw, hostname, ns, rootID, ok := knownSubdomainDetails(r.Host, knownGateways); ok {
				// Looks like we're using known subdomain gateway.

				// Assemble original path prefix.
				pathPrefix := "/" + ns + "/" + rootID

				// Does this gateway _handle_ this path?
				if !(gw.UseSubdomains && hasPrefix(pathPrefix, gw.Paths...)) {
					// If not, resource does not exist, return 404
					http.NotFound(w, r)
					return
				}

				// Do we need to fix multicodec in PeerID represented as CIDv1?
				if isPeerIDNamespace(ns) {
					keyCid, err := cid.Decode(rootID)
					if err == nil && keyCid.Type() != cid.Libp2pKey {
						if newURL, ok := toSubdomainURL(hostname, pathPrefix+r.URL.Path, r); ok {
							// Redirect to CID fixed inside of toSubdomainURL()
							http.Redirect(w, r, newURL, http.StatusMovedPermanently)
							return
						}
					}
				}

				// Rewrite the path to not use subdomains
				r.URL.Path = pathPrefix + r.URL.Path

				// Serve path request
				childMux.ServeHTTP(w, r)
				return
			}
			// We don't have a known gateway. Fallback on DNSLink lookup

			// Wildcard HTTP Host check:
			// 1. is wildcard DNSLink enabled (Gateway.NoDNSLink=false)?
			// 2. does Host header include a fully qualified domain name (FQDN)?
			// 3. does DNSLink record exist in DNS?
			if !cfg.Gateway.NoDNSLink && isDNSLinkRequest(n.Context(), coreApi, r) {
				// rewrite path and handle as DNSLink
				r.URL.Path = "/ipns/" + stripPort(r.Host) + r.URL.Path
				childMux.ServeHTTP(w, r)
				return
			}

			// else, treat it as an old school gateway, I guess.
			childMux.ServeHTTP(w, r)
		})
		return childMux, nil
	}
}

// isKnownHostname checks Gateway.PublicGateways and returns matching
// GatewaySpec with gracefull fallback to version without port
func isKnownHostname(hostname string, knownGateways map[string]config.GatewaySpec) (gw config.GatewaySpec, ok bool) {
	// Try hostname (host+optional port - value from Host header as-is)
	if gw, ok := knownGateways[hostname]; ok {
		return gw, ok
	}
	// Fallback to hostname without port
	gw, ok = knownGateways[stripPort(hostname)]
	return gw, ok
}

// Parses Host header and looks for a known subdomain gateway host.
// If found, returns GatewaySpec and subdomain components.
// Note: hostname is host + optional port
func knownSubdomainDetails(hostname string, knownGateways map[string]config.GatewaySpec) (gw config.GatewaySpec, knownHostname, ns, rootID string, ok bool) {
	labels := strings.Split(hostname, ".")
	// Look for FQDN of a known gateway hostname.
	// Example: given "dist.ipfs.io.ipns.dweb.link":
	// 1. Lookup "link" TLD in knownGateways: negative
	// 2. Lookup "dweb.link" in knownGateways: positive
	//
	// Stops when we have 2 or fewer labels left as we need at least a
	// rootId and a namespace.
	for i := len(labels) - 1; i >= 2; i-- {
		fqdn := strings.Join(labels[i:], ".")
		gw, ok := isKnownHostname(fqdn, knownGateways)
		if !ok {
			continue
		}

		ns := labels[i-1]
		if !isSubdomainNamespace(ns) {
			break
		}

		// Merge remaining labels (could be a FQDN with DNSLink)
		rootID := strings.Join(labels[:i-1], ".")
		return gw, fqdn, ns, rootID, true
	}
	// not a known subdomain gateway
	return gw, "", "", "", false
}

// isDNSLinkRequest returns bool that indicates if request
// should return data from content path listed in DNSLink record (if exists)
func isDNSLinkRequest(ctx context.Context, ipfs iface.CoreAPI, r *http.Request) bool {
	fqdn := stripPort(r.Host)
	if len(fqdn) == 0 && !isd.IsDomain(fqdn) {
		return false
	}
	name := "/ipns/" + fqdn
	// check if DNSLink exists
	depth := options.Name.ResolveOption(nsopts.Depth(1))
	_, err := ipfs.Name().Resolve(ctx, name, depth)
	return err == nil || err == namesys.ErrResolveRecursion
}

func isSubdomainNamespace(ns string) bool {
	switch ns {
	case "ipfs", "ipns", "p2p", "ipld":
		return true
	default:
		return false
	}
}

func isPeerIDNamespace(ns string) bool {
	switch ns {
	case "ipns", "p2p":
		return true
	default:
		return false
	}
}

// Converts a hostname/path to a subdomain-based URL, if applicable.
func toSubdomainURL(hostname, path string, r *http.Request) (redirURL string, ok bool) {
	var scheme, ns, rootID, rest string

	query := r.URL.RawQuery
	parts := strings.SplitN(path, "/", 4)
	safeRedirectURL := func(in string) (out string, ok bool) {
		safeURI, err := url.ParseRequestURI(in)
		if err != nil {
			return "", false
		}
		return safeURI.String(), true
	}

	// Support X-Forwarded-Proto if added by a reverse proxy
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-Proto
	xproto := r.Header.Get("X-Forwarded-Proto")
	if xproto == "https" {
		scheme = "https:"
	} else {
		scheme = "http:"
	}

	switch len(parts) {
	case 4:
		rest = parts[3]
		fallthrough
	case 3:
		ns = parts[1]
		rootID = parts[2]
	default:
		return "", false
	}

	if !isSubdomainNamespace(ns) {
		return "", false
	}

	// add prefix if query is present
	if query != "" {
		query = "?" + query
	}

	// Normalize problematic PeerIDs (eg. ed25519+identity) to CID representation
	if isPeerIDNamespace(ns) && !isd.IsDomain(rootID) {
		peerID, err := peer.Decode(rootID)
		// Note: PeerID CIDv1 with protobuf multicodec will fail, but we fix it
		// in the next block
		if err == nil {
			rootID = peer.ToCid(peerID).String()
		}
	}

	// If rootID is a CID, ensure it uses DNS-friendly text representation
	if rootCid, err := cid.Decode(rootID); err == nil {
		multicodec := rootCid.Type()

		// PeerIDs represented as CIDv1 are expected to have libp2p-key
		// multicodec (https://github.com/libp2p/specs/pull/209).
		// We ease the transition by fixing multicodec on the fly:
		// https://github.com/ipfs/go-ipfs/issues/5287#issuecomment-492163929
		if isPeerIDNamespace(ns) && multicodec != cid.Libp2pKey {
			multicodec = cid.Libp2pKey
		}

		// if object turns out to be a valid CID,
		// ensure text representation used in subdomain is CIDv1 in Base32
		// https://github.com/ipfs/in-web-browsers/issues/89
		rootID, err = cid.NewCidV1(multicodec, rootCid.Hash()).StringOfBase(mbase.Base32)
		if err != nil {
			// should not error, but if it does, its clealy not possible to
			// produce a subdomain URL
			return "", false
		}
	}

	return safeRedirectURL(fmt.Sprintf(
		"%s//%s.%s.%s/%s%s",
		scheme,
		rootID,
		ns,
		hostname,
		rest,
		query,
	))
}

func hasPrefix(path string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		// Assume people are creative with trailing slashes in Gateway config
		p := strings.TrimSuffix(prefix, "/")
		// Support for both /version and /ipfs/$cid
		if p == path || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}

func stripPort(hostname string) string {
	host, _, err := net.SplitHostPort(hostname)
	if err == nil {
		return host
	}
	return hostname
}
