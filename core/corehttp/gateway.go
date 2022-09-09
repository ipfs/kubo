package corehttp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sort"

	coreiface "github.com/ipfs/interface-go-ipfs-core"
	options "github.com/ipfs/interface-go-ipfs-core/options"
	path "github.com/ipfs/interface-go-ipfs-core/path"
	version "github.com/ipfs/kubo"
	core "github.com/ipfs/kubo/core"
	coreapi "github.com/ipfs/kubo/core/coreapi"
	id "github.com/libp2p/go-libp2p/p2p/protocol/identify"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type GatewayConfig struct {
	Headers               map[string][]string
	Writable              bool
	FastDirIndexThreshold int
}

// NodeAPI defines the minimal set of API services required by a gateway handler
type NodeAPI interface {
	// Unixfs returns an implementation of Unixfs API
	Unixfs() coreiface.UnixfsAPI

	// Block returns an implementation of Block API
	Block() coreiface.BlockAPI

	// Dag returns an implementation of Dag API
	Dag() coreiface.APIDagService

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

func GatewayOption(writable bool, paths ...string) ServeOption {
	return func(n *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		cfg, err := n.Repo.Config()
		if err != nil {
			return nil, err
		}

		api, err := coreapi.NewCoreAPI(n, options.Api.FetchBlocks(!cfg.Gateway.NoFetch))
		if err != nil {
			return nil, err
		}

		headers := make(map[string][]string, len(cfg.Gateway.HTTPHeaders))
		for h, v := range cfg.Gateway.HTTPHeaders {
			headers[http.CanonicalHeaderKey(h)] = v
		}

		AddAccessControlHeaders(headers)

		offlineApi, err := api.WithOptions(options.Api.Offline(true))
		if err != nil {
			return nil, err
		}

		gateway := NewGatewayHandler(GatewayConfig{
			Headers:               headers,
			Writable:              writable,
			FastDirIndexThreshold: int(cfg.Gateway.FastDirIndexThreshold.WithDefault(100)),
		}, api, offlineApi)

		gateway = otelhttp.NewHandler(gateway, "Gateway.Request")

		for _, p := range paths {
			mux.Handle(p+"/", gateway)
		}
		return mux, nil
	}
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

func VersionOption() ServeOption {
	return func(_ *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Commit: %s\n", version.CurrentCommit)
			fmt.Fprintf(w, "Client Version: %s\n", version.GetUserAgentVersion())
			fmt.Fprintf(w, "Protocol Version: %s\n", id.LibP2PVersion)
		})
		return mux, nil
	}
}
