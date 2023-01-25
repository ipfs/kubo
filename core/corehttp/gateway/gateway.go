package gateway

import (
	"context"
	"net/http"
	"sort"

	coreiface "github.com/ipfs/interface-go-ipfs-core"
	path "github.com/ipfs/interface-go-ipfs-core/path"
)

// Config is the configuration that will be applied when creating a new gateway
// handler.
type Config struct {
	Headers  map[string][]string
	Writable bool
}

// NodeAPI defines the minimal set of API services required by a gateway handler
type NodeAPI interface {
	// Unixfs returns an implementation of Unixfs API
	Unixfs() coreiface.UnixfsAPI

	// Block returns an implementation of Block API
	Block() coreiface.BlockAPI

	// Dag returns an implementation of Dag API
	Dag() coreiface.APIDagService

	// Routing returns an implementation of Routing API.
	// Used for returning signed IPNS records, see IPIP-0328
	Routing() coreiface.RoutingAPI

	// ResolvePath resolves the path using Unixfs resolver
	ResolvePath(context.Context, path.Path) (path.Resolved, error)
}

// A helper function to clean up a set of headers:
// 1. Canonicalizes.
// 2. Deduplicates.
// 3. Sorts.
func cleanHeaderSet(headers []string) []string {
	// Deduplicate and canonicalize.
	m := make(map[string]struct{}, len(headers))
	for _, h := range headers {
		m[http.CanonicalHeaderKey(h)] = struct{}{}
	}
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}

	// Sort
	sort.Strings(result)
	return result
}

// AddAccessControlHeaders adds default headers used for controlling
// cross-origin requests. This function adds several values to the
// Access-Control-Allow-Headers and Access-Control-Expose-Headers entries.
// If the Access-Control-Allow-Origin entry is missing a value of '*' is
// added, indicating that browsers should allow requesting code from any
// origin to access the resource.
// If the Access-Control-Allow-Methods entry is missing a value of 'GET' is
// added, indicating that browsers may use the GET method when issuing cross
// origin requests.
func AddAccessControlHeaders(headers map[string][]string) {
	// Hard-coded headers.
	const ACAHeadersName = "Access-Control-Allow-Headers"
	const ACEHeadersName = "Access-Control-Expose-Headers"
	const ACAOriginName = "Access-Control-Allow-Origin"
	const ACAMethodsName = "Access-Control-Allow-Methods"

	if _, ok := headers[ACAOriginName]; !ok {
		// Default to *all*
		headers[ACAOriginName] = []string{"*"}
	}
	if _, ok := headers[ACAMethodsName]; !ok {
		// Default to GET
		headers[ACAMethodsName] = []string{http.MethodGet}
	}

	headers[ACAHeadersName] = cleanHeaderSet(
		append([]string{
			"Content-Type",
			"User-Agent",
			"Range",
			"X-Requested-With",
		}, headers[ACAHeadersName]...))

	headers[ACEHeadersName] = cleanHeaderSet(
		append([]string{
			"Content-Length",
			"Content-Range",
			"X-Chunked-Output",
			"X-Stream-Output",
			"X-Ipfs-Path",
			"X-Ipfs-Roots",
		}, headers[ACEHeadersName]...))
}

type RequestContextKey string

const (
	DNSLinkHostnameKey RequestContextKey = "dnslink-hostname"
	GatewayHostnameKey RequestContextKey = "gw-hostname"
)
