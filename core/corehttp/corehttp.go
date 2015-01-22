package corehttp

import (
	"fmt"
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

func ListenAndServe(n *core.IpfsNode, addr ma.Multiaddr, options ...ServeOption) error {
	mux := http.NewServeMux()
	for _, option := range options {
		if err := option(n, mux); err != nil {
			return err
		}
	}
	return listenAndServe("API", n, addr, mux)
}

func listenAndServe(name string, node *core.IpfsNode, addr ma.Multiaddr, mux *http.ServeMux) error {
	_, host, err := manet.DialArgs(addr)
	if err != nil {
		return err
	}

	server := manners.NewServer()

	// if the server exits beforehand
	var serverError error
	serverExited := make(chan struct{})

	go func() {
		fmt.Printf("%s server listening on %s\n", name, addr)
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
