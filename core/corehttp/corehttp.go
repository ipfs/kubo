package corehttp

import (
	"fmt"
	"net/http"
	"os"

	manners "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/braintree/manners"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
	commands "github.com/jbenet/go-ipfs/commands"
	cmdsHttp "github.com/jbenet/go-ipfs/commands/http"
	core "github.com/jbenet/go-ipfs/core"
	corecommands "github.com/jbenet/go-ipfs/core/commands"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
)

var log = eventlog.Logger("core/server")

const (
	// TODO rename
	originEnvKey = "API_ORIGIN"
	webuiPath    = "/ipfs/QmTWvqK9dYvqjAMAcCeUun8b45Fwu7wPhEN9B9TsGbkXfJ"
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

func CommandsOption(cctx commands.Context) ServeOption {
	return func(n *core.IpfsNode, mux *http.ServeMux) error {
		origin := os.Getenv(originEnvKey)
		cmdHandler := cmdsHttp.NewHandler(cctx, corecommands.Root, origin)
		mux.Handle(cmdsHttp.ApiPath+"/", cmdHandler)
		return nil
	}
}

func GatewayOption(n *core.IpfsNode, mux *http.ServeMux) error {
	gateway, err := newGatewayHandler(n)
	if err != nil {
		return err
	}
	mux.Handle("/ipfs/", gateway)
	return nil
}

func WebUIOption(n *core.IpfsNode, mux *http.ServeMux) error {
	mux.Handle("/webui/", &redirectHandler{webuiPath})
	return nil
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

type redirectHandler struct {
	path string
}

func (i *redirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, i.path, 302)
}
