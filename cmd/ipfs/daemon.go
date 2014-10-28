package main

import (
	"fmt"
	"net/http"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr/net"

	cmds "github.com/jbenet/go-ipfs/commands"
	cmdsHttp "github.com/jbenet/go-ipfs/commands/http"
	"github.com/jbenet/go-ipfs/daemon"
)

var Daemon = &cmds.Command{
	Options:     []cmds.Option{},
	Help:        "TODO",
	Subcommands: map[string]*cmds.Command{},
	Run:         daemonFunc,
}

func daemonFunc(req cmds.Request, res cmds.Response) {
	// TODO: spin up a core.IpfsNode

	ctx := req.Context()

	lk, err := daemon.Lock(ctx.ConfigRoot)
	if err != nil {
		res.SetError(fmt.Errorf("Couldn't obtain lock. Is another daemon already running?"), cmds.ErrNormal)
		return
	}
	defer lk.Close()

	addr, err := ma.NewMultiaddr(ctx.Config.Addresses.API)
	if err != nil {
		res.SetError(err, cmds.ErrNormal)
		return
	}

	_, host, err := manet.DialArgs(addr)
	if err != nil {
		res.SetError(err, cmds.ErrNormal)
		return
	}

	handler := cmdsHttp.Handler{*ctx}
	http.Handle(cmdsHttp.ApiPath+"/", handler)
	err = http.ListenAndServe(host, nil)
	if err != nil {
		res.SetError(err, cmds.ErrNormal)
		return
	}
	// TODO: log to indicate that we are now listening

}
