package corehttp

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"

	core "github.com/ipfs/go-ipfs/core"

	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	peer "gx/ipfs/QmbNepETomvmXfz1X5pHNFD2QuPqnqi47dTd94QJWSorQ3/go-libp2p-peer"
	inet "gx/ipfs/QmfDPh144WGBqRxZb1TGDHerbMnZATrHZggAPw7putNnBq/go-libp2p-net"
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

			// open connect to peer
			stream, err := ipfsNode.P2P.PeerHost.NewStream(request.Context(), parsedRequest.target, protocol.ID("/x/"+parsedRequest.name))
			if err != nil {
				msg := fmt.Sprintf("Failed to open stream '%v' to target peer '%v'", parsedRequest.name, parsedRequest.target)
				handleError(w, msg, err, 500)
				return
			}
			//send proxy request and response to client
			newReverseHTTPProxy(parsedRequest, stream).ServeHTTP(w, request)
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

	peerID, err := peer.IDB58Decode(split[3])

	if err != nil {
		return nil, err
	}

	return &proxyRequest{peerID, split[4], "/" + split[5]}, nil
}

func handleError(w http.ResponseWriter, msg string, err error, code int) {
	w.WriteHeader(code)
	fmt.Fprintf(w, "%s: %s\n", msg, err)
	log.Warningf("server error: %s: %s", err)
}

func newReverseHTTPProxy(req *proxyRequest, streamToPeer inet.Stream) *httputil.ReverseProxy {
	director := func(r *http.Request) {
		r.URL.Path = req.httpPath //the scheme etc. doesn't matter
	}

	return &httputil.ReverseProxy{
		Director:  director,
		Transport: &roundTripper{streamToPeer}}
}

type roundTripper struct {
	stream inet.Stream
}

// we wrap the response body and close the stream
// only when it's closed.
type respBody struct {
	io.ReadCloser
	stream inet.Stream
}

// Closes the response's body and the connection.
func (rb *respBody) Close() error {
	if err := rb.stream.Close(); err != nil {
		rb.stream.Reset()
	} else {
		go inet.AwaitEOF(rb.stream)
	}
	return rb.ReadCloser.Close()
}

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {

	sendRequest := func() {
		err := req.Write(rt.stream)
		if err != nil {
			rt.stream.Close()
		}
		if req.Body != nil {
			req.Body.Close()
		}
	}
	//send request while reading response
	go sendRequest()
	s := bufio.NewReader(rt.stream)

	resp, err := http.ReadResponse(s, req)
	if err != nil {
		return resp, err
	}

	resp.Body = &respBody{
		ReadCloser: resp.Body,
		stream:     rt.stream,
	}

	return resp, nil
}
