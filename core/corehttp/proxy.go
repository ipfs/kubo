package corehttp

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	core "github.com/ipfs/go-ipfs/core"

	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	p2phttp "gx/ipfs/QmcLYfmHLsaVRKGMZQovwEYhHAjWtRjg1Lij3pnzw5UkRD/go-libp2p-http"
)

// ProxyOption is an endpoint for proxying a HTTP request to another ipfs peer
func ProxyOption() ServeOption {
	return func(ipfsNode *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		mux.HandleFunc("/proxy/http/", func(w http.ResponseWriter, request *http.Request) {
			// parse request
			parsedRequest, err := parseRequest(request)
			if err != nil {
				handleError(w, "Failed to parse request", err, 400)
				return
			}

			target, err := url.Parse(fmt.Sprintf("libp2p://%s/%s", parsedRequest.target, parsedRequest.httpPath))
			if err != nil {
				handleError(w, "Failed to parse url", err, 400)
				return
			}

			rt := p2phttp.NewTransport(ipfsNode.P2P.PeerHost, p2phttp.ProtocolOption(parsedRequest.name))
			proxy := httputil.NewSingleHostReverseProxy(target)
			proxy.Transport = rt
			proxy.ServeHTTP(w, request)
		})
		return mux, nil
	}
}

type proxyRequest struct {
	target   string
	name     protocol.ID
	httpPath string // path to send to the proxy-host
}

// from the url path parse the peer-ID, name and http path
// /proxy/http/$peer_id/$name/$http_path
func parseRequest(request *http.Request) (*proxyRequest, error) {
	path := request.URL.Path

	split := strings.SplitN(path, "/", 6)
	if len(split) < 6 {
		return nil, fmt.Errorf("Invalid request path '%s'", path)
	}

	return &proxyRequest{split[3], protocol.ID(split[4]), "/" + split[5]}, nil
}

func handleError(w http.ResponseWriter, msg string, err error, code int) {
	w.WriteHeader(code)
	fmt.Fprintf(w, "%s: %s\n", msg, err)
	log.Warningf("server error: %s: %s", err)
}
