package corehttp

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	core "github.com/ipfs/go-ipfs/core"

	protocol "github.com/libp2p/go-libp2p-core/protocol"
	p2phttp "github.com/libp2p/go-libp2p-http"
)

// P2PProxyOption is an endpoint for proxying a HTTP request to another ipfs peer
func P2PProxyOption() ServeOption {
	return func(ipfsNode *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		mux.HandleFunc("/p2p/", func(w http.ResponseWriter, request *http.Request) {
			// parse request
			parsedRequest, err := parseRequest(request)
			if err != nil {
				handleError(w, "failed to parse request", err, 400)
				return
			}

			request.Host = "" // Let URL's Host take precedence.
			request.URL.Path = parsedRequest.httpPath
			target, err := url.Parse(fmt.Sprintf("libp2p://%s", parsedRequest.target))
			if err != nil {
				handleError(w, "failed to parse url", err, 400)
				return
			}

			rt := p2phttp.NewTransport(ipfsNode.PeerHost, p2phttp.ProtocolOption(parsedRequest.name))
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
// /p2p/$peer_id/http/$http_path
// or
// /p2p/$peer_id/x/$protocol/http/$http_path
func parseRequest(request *http.Request) (*proxyRequest, error) {
	path := request.URL.Path

	split := strings.SplitN(path, "/", 5)
	if len(split) < 5 {
		return nil, fmt.Errorf("Invalid request path '%s'", path)
	}

	if split[3] == "http" {
		return &proxyRequest{split[2], protocol.ID("/http"), split[4]}, nil
	}

	split = strings.SplitN(path, "/", 7)
	if split[3] != "x" || split[5] != "http" {
		return nil, fmt.Errorf("Invalid request path '%s'", path)
	}

	return &proxyRequest{split[2], protocol.ID("/x/" + split[4] + "/http"), split[6]}, nil
}

func handleError(w http.ResponseWriter, msg string, err error, code int) {
	http.Error(w, fmt.Sprintf("%s: %s", msg, err), code)
}
