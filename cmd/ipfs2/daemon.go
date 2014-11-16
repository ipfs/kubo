package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr/net"

	manners "github.com/braintree/manners"
	cmds "github.com/jbenet/go-ipfs/commands"
	cmdsHttp "github.com/jbenet/go-ipfs/commands/http"
	core "github.com/jbenet/go-ipfs/core"
	commands "github.com/jbenet/go-ipfs/core/commands2"
	daemon "github.com/jbenet/go-ipfs/daemon2"
)

var daemonCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Run a network-connected IPFS node",
		ShortDescription: `
'ipfs daemon' runs a persistent IPFS daemon that can serve commands
over the network. Most applications that use IPFS will do so by
communicating with a daemon over the HTTP API. While the daemon is
running, calls to 'ipfs' commands will be sent over the network to
the daemon.
`,
	},

	Options:     []cmds.Option{},
	Subcommands: map[string]*cmds.Command{},
	Run:         daemonFunc,
}

func daemonFunc(req cmds.Request) (interface{}, error) {
	lock, err := daemon.Lock(req.Context().ConfigRoot)
	if err != nil {
		return nil, fmt.Errorf("Couldn't obtain lock. Is another daemon already running?")
	}
	defer lock.Close()

	cfg, err := req.Context().GetConfig()
	if err != nil {
		return nil, err
	}

	// setup function that constructs the context. we have to do it this way
	// to play along with how the Context works and thus not expose its internals
	req.Context().ConstructNode = func() (*core.IpfsNode, error) {
		return core.NewIpfsNode(cfg, true)
	}
	node, err := req.Context().GetNode()
	if err != nil {
		return nil, err
	}

	addr, err := ma.NewMultiaddr(cfg.Addresses.API)
	if err != nil {
		return nil, err
	}

	_, host, err := manet.DialArgs(addr)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	cmdHandler := cmdsHttp.NewHandler(*req.Context(), commands.Root)
	mux.Handle(cmdsHttp.ApiPath+"/", cmdHandler)

	ifpsHandler := &ipfsHandler{node}
	mux.Handle("/ipfs/", ifpsHandler)

	err = listenAndServe(mux, host)
	return nil, err
}

func listenAndServe(mux *http.ServeMux, host string) error {

	fmt.Printf("API server listening on '%s'\n", host)
	s := manners.NewServer()
	done := make(chan struct{}, 1)
	defer func() {
		done <- struct{}{}
	}()

	// go wait until we kill it.
	go func() {
		sig := sigTerm()
		select {
		case <-done:
			log.Info("daemon terminated at %s.", host)
		case <-sig:
			s.Shutdown <- true
			log.Info("terminating daemon at %s...", host)
		}
	}()

	if err := s.ListenAndServe(host, mux); err != nil {
		return err
	}
	return nil
}

func sigTerm() chan os.Signal {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGHUP, syscall.SIGINT,
		syscall.SIGTERM, syscall.SIGQUIT)
	return sigc
}
