package corehttp

import (
	"net/http"

	manners "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/braintree/manners"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
	core "github.com/jbenet/go-ipfs/core"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
)

var log = eventlog.Logger("core/server")

const (
// TODO rename
)

type ServeOption func(*core.IpfsNode, *http.ServeMux) error

// ListenAndServe runs an HTTP server listening at |listeningMultiAddr| with
// the given serve options. The address must be provided in multiaddr format.
//
// TODO intelligently parse address strings in other formats so long as they
// unambiguously map to a valid multiaddr. e.g. for convenience, ":8080" should
// map to "/ip4/0.0.0.0/tcp/8080".
func ListenAndServe(n *core.IpfsNode, listeningMultiAddr string, options ...ServeOption) error {
	addr, err := ma.NewMultiaddr(listeningMultiAddr)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	for _, option := range options {
		if err := option(n, mux); err != nil {
			return err
		}
	}
	return listenAndServe(n, addr, mux)
}

func listenAndServe(node *core.IpfsNode, addr ma.Multiaddr, mux *http.ServeMux) error {
	_, host, err := manet.DialArgs(addr)
	if err != nil {
		return err
	}

	server := manners.NewServer()

	// if the server exits beforehand
	var serverError error
	serverExited := make(chan struct{})

	go func() {
		serverError = server.ListenAndServe(host, mux)
		close(serverExited)
	}()

	// wait for server to exit.
	select {
	case <-serverExited:

	// if node being closed before server exits, close server
	case <-node.Closing():
		log.Infof("server at %s terminating...", addr)
		server.Shutdown <- true
		<-serverExited // now, DO wait until server exit
	}

	log.Infof("server at %s terminated", addr)
	return serverError
}
