package corehttp

import (
	"fmt"
	"net"
	"net/http"
	"sort"

	version "github.com/ipfs/go-ipfs"
	core "github.com/ipfs/go-ipfs/core"
	coreapi "github.com/ipfs/go-ipfs/core/coreapi"

	options "github.com/ipfs/interface-go-ipfs-core/options"
	id "github.com/libp2p/go-libp2p/p2p/protocol/identify"
)

type GatewayConfig struct {
	Headers      map[string][]string
	Writable     bool
	PathPrefixes []string
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
				"Content-Range",
				"X-Chunked-Output",
				"X-Stream-Output",
			}, headers[ACEHeadersName]...))

		gateway := newGatewayHandler(GatewayConfig{
			Headers:      headers,
			Writable:     writable,
			PathPrefixes: cfg.Gateway.PathPrefixes,
		}, api)

		for _, p := range paths {
			mux.Handle(p+"/", gateway)
		}
		return mux, nil
	}
}

func VersionOption() ServeOption {
	return func(_ *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Commit: %s\n", version.CurrentCommit)
			fmt.Fprintf(w, "Client Version: %s\n", version.UserAgent)
			fmt.Fprintf(w, "Protocol Version: %s\n", id.LibP2PVersion)
		})
		return mux, nil
	}
}
