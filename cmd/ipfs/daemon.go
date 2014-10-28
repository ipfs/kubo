package main

import (
	"fmt"
	"net/http"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/camlistore/lock"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr/net"

	cmds "github.com/jbenet/go-ipfs/commands"
	cmdsHttp "github.com/jbenet/go-ipfs/commands/http"
	"github.com/jbenet/go-ipfs/config"
)

// DaemonLockFile is the filename of the daemon lock, relative to config dir
const DaemonLockFile = "daemon.lock"

var Daemon = &cmds.Command{
	Options:     []cmds.Option{},
	Help:        "TODO",
	Subcommands: map[string]*cmds.Command{},
	Run:         daemonFunc,
}

func daemonFunc(req cmds.Request, res cmds.Response) {
	configPath, err := getConfigPath(req)
	if err != nil {
		res.SetError(err, cmds.ErrNormal)
		return
	}

	lockPath, err := config.Path(configPath, DaemonLockFile)
	if err != nil {
		res.SetError(err, cmds.ErrNormal)
		return
	}

	lk, err := lock.Lock(lockPath)
	if err != nil {
		res.SetError(fmt.Errorf("Couldn't obtain lock. Is another daemon already running?"), cmds.ErrNormal)
		return
	}
	defer lk.Close()

	configFile, err := config.Filename(configPath)
	if err != nil {
		res.SetError(err, cmds.ErrNormal)
		return
	}

	config, err := config.Load(configFile)
	if err != nil {
		res.SetError(err, cmds.ErrNormal)
		return
	}

	addr, err := ma.NewMultiaddr(config.Addresses.API)
	if err != nil {
		res.SetError(err, cmds.ErrNormal)
		return
	}

	_, host, err := manet.DialArgs(addr)
	if err != nil {
		res.SetError(err, cmds.ErrNormal)
		return
	}

	handler := cmdsHttp.Handler{}
	http.Handle(cmdsHttp.ApiPath+"/", handler)
	err = http.ListenAndServe(host, nil)
	if err != nil {
		res.SetError(err, cmds.ErrNormal)
		return
	}
	// TODO: log to indicate that we are now listening

}
