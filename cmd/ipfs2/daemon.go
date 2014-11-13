package main

import (
	"fmt"
	"net/http"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr/net"

	cmds "github.com/jbenet/go-ipfs/commands"
	cmdsHttp "github.com/jbenet/go-ipfs/commands/http"
	core "github.com/jbenet/go-ipfs/core"
	commands "github.com/jbenet/go-ipfs/core/commands2"
	daemon "github.com/jbenet/go-ipfs/daemon2"
)

var daemonCmd = &cmds.Command{
	Options:     []cmds.Option{},
	Help:        "run a network-connected ipfs node", // TODO adjust copy
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

	cmdHandler := cmdsHttp.NewHandler(*req.Context(), commands.Root)
	http.Handle(cmdsHttp.ApiPath+"/", cmdHandler)

	ifpsHandler := &ipfsHandler{node}
	http.Handle("/ipfs/", ifpsHandler)

	fmt.Printf("API server listening on '%s'\n", host)

	err = http.ListenAndServe(host, nil)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
