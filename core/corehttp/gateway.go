package corehttp

import (
	"fmt"
	"net"
	"net/http"

	options "github.com/ipfs/interface-go-ipfs-core/options"
	version "github.com/ipfs/kubo"
	core "github.com/ipfs/kubo/core"
	coreapi "github.com/ipfs/kubo/core/coreapi"
	"github.com/ipfs/kubo/core/corehttp/gateway"
	id "github.com/libp2p/go-libp2p/p2p/protocol/identify"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

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

		gateway.AddAccessControlHeaders(headers)

		offlineAPI, err := api.WithOptions(options.Api.Offline(true))
		if err != nil {
			return nil, err
		}

		gateway := gateway.NewHandler(gateway.Config{
			Headers:  headers,
			Writable: writable,
		}, api, offlineAPI)

		gateway = otelhttp.NewHandler(gateway, "Gateway.Request")

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
			fmt.Fprintf(w, "Client Version: %s\n", version.GetUserAgentVersion())
			fmt.Fprintf(w, "Protocol Version: %s\n", id.DefaultProtocolVersion)
		})
		return mux, nil
	}
}
