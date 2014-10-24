package main

import (
	"net/http"

	cmds "github.com/jbenet/go-ipfs/commands"
	cmdsHttp "github.com/jbenet/go-ipfs/commands/http"
)

var Daemon = &cmds.Command{
	Options:     []cmds.Option{},
	Help:        "TODO",
	Subcommands: map[string]*cmds.Command{},
	Run:         daemonFunc,
}

func daemonFunc(req cmds.Request, res cmds.Response) {
	handler := cmdsHttp.Handler{}
	http.Handle(cmdsHttp.ApiPath+"/", handler)
	// TODO: load listen address/port from config/options
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		res.SetError(err, cmds.ErrNormal)
		return
	}
	// TODO: log to indicate that we are now listening
}
