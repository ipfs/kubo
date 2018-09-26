package corehttp

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strings"

	core "github.com/ipfs/go-ipfs/core"

	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	peer "gx/ipfs/QmbNepETomvmXfz1X5pHNFD2QuPqnqi47dTd94QJWSorQ3/go-libp2p-peer"
)

func ProxyOption() ServeOption {
	return func(ipfsNode *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		mux.HandleFunc("/proxy/", func(w http.ResponseWriter, request *http.Request) {
			// parse request
			parsedRequest, err := parseRequest(request)
			if err != nil {
				handleError(w, "Failed to parse request", err, 400)
				return
			}

			// open connect to peer
			stream, err := ipfsNode.P2P.PeerHost.NewStream(ipfsNode.Context(), parsedRequest.target, protocol.ID("/x/"+parsedRequest.name))
			if err != nil {
				msg := fmt.Sprintf("Failed to open stream '%v' to target peer '%v'", parsedRequest.name, parsedRequest.target)
				handleError(w, msg, err, 500)
				return
			}

			// send request to peer
			proxyReq, err := http.NewRequest(request.Method, parsedRequest.httpPath, request.Body)

			if err != nil {
				handleError(w, "Failed to format proxy request", err, 500)
				return
			}

			proxyReq.Write(stream)

			s := bufio.NewReader(stream)
			proxyResponse, err := http.ReadResponse(s, proxyReq)
			defer func() { proxyResponse.Body.Close() }()
			if err != nil {
				msg := fmt.Sprintf("Failed to send request to stream '%v' to peer '%v'", parsedRequest.name, parsedRequest.target)
				handleError(w, msg, err, 500)
				return
			}
			// send client response
			proxyResponse.Write(w)
		})
		return mux, nil
	}
}

type proxyRequest struct {
	target   peer.ID
	name     string
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

	if split[2] != "http" {
		return nil, fmt.Errorf("Invalid proxy request protocol '%s'", split[2])
	}

	peerID, err := peer.IDB58Decode(split[3])

	if err != nil {
		return nil, err
	}

	return &proxyRequest{peerID, split[4], split[5]}, nil
}

// log error and send response to client
func handleError(w http.ResponseWriter, msg string, err error, code int) {
	w.WriteHeader(code)
	fmt.Fprintf(w, "%s: %s\n", msg, err)
	log.Warningf("server error: %s: %s", err)
}
