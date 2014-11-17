package main

import (
	"fmt"
	"net/http"

	manners "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/braintree/manners"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr/net"
	cmds "github.com/jbenet/go-ipfs/commands"
	cmdsHttp "github.com/jbenet/go-ipfs/commands/http"
	core "github.com/jbenet/go-ipfs/core"
	commands "github.com/jbenet/go-ipfs/core/commands2"
	daemon "github.com/jbenet/go-ipfs/daemon2"
	util "github.com/jbenet/go-ipfs/util"
	errors "github.com/jbenet/go-ipfs/util/errors"
)

const (
	initOptionKwd = "init"
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

	Options: []cmds.Option{
		cmds.BoolOption(initOptionKwd, "Initialize IPFS with default settings if not already initialized"),
	},
	Subcommands: map[string]*cmds.Command{},
	Run:         daemonFunc,
}

func daemonFunc(req cmds.Request) (interface{}, error) {

	initialize, _, err := req.Option(initOptionKwd).Bool()
	if err != nil {
		return nil, err
	}
	if initialize {

		// now, FileExists is our best method of detecting whether IPFS is
		// configured. Consider moving this into a config helper method
		// `IsInitialized` where the quality of the signal can be improved over
		// time, and many call-sites can benefit.
		if !util.FileExists(req.Context().ConfigRoot) {
			err := initWithDefaults(req.Context().ConfigRoot)
			if err != nil {
				return nil, errors.Wrap(err)
			}
		}
	}

	lock, err := daemon.Lock(req.Context().ConfigRoot)
	if err != nil {
		return nil, errors.Errorf("Couldn't obtain lock. Is another daemon already running?")
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

	err = listenAndServe(node, mux, host)
	return nil, err
}

func listenAndServe(node *core.IpfsNode, mux *http.ServeMux, host string) error {

	fmt.Printf("API server listening on '%s'\n", host)
	s := manners.NewServer()

	done := make(chan struct{}, 1)
	defer func() {
		done <- struct{}{}
	}()

	// go wait until the node dies
	go func() {
		select {
		case <-node.Closed():
		case <-done:
			return
		}

		log.Info("terminating daemon at %s...", host)
		s.Shutdown <- true
	}()

	if err := s.ListenAndServe(host, mux); err != nil {
		return err
	}

	return nil
}
