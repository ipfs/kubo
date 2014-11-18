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
	"github.com/jbenet/go-ipfs/util/debugerror"
)

const (
	initOptionKwd = "init"
	mountKwd      = "mount"
	ipfsMountKwd  = "mount-ipfs"
	ipnsMountKwd  = "mount-ipns"
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
		cmds.BoolOption(mountKwd, "Mounts IPFS to the filesystem"),
		cmds.StringOption(ipfsMountKwd, "Path to the mountpoint for IPFS (if using --mount)"),
		cmds.StringOption(ipnsMountKwd, "Path to the mountpoint for IPNS (if using --mount)"),
	},
	Subcommands: map[string]*cmds.Command{},
	Run:         daemonFunc,
}

func daemonFunc(req cmds.Request) (interface{}, error) {
	cfg, err := req.Context().GetConfig()
	if err != nil {
		return nil, err
	}

	node, err := core.NewIpfsNode(cfg, true)
	if err != nil {
		return nil, err
	}

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
				return nil, debugerror.Wrap(err)
			}
		}
	}

	lock, err := daemon.Lock(req.Context().ConfigRoot)
	if err != nil {
		return nil, debugerror.Errorf("Couldn't obtain lock. Is another daemon already running?")
	}
	defer lock.Close()

	addr, err := ma.NewMultiaddr(cfg.Addresses.API)
	if err != nil {
		return nil, err
	}

	_, host, err := manet.DialArgs(addr)
	if err != nil {
		return nil, err
	}

	// mount if the user provided the --mount flag
	mount, _, err := req.Option(mountKwd).Bool()
	if err != nil {
		return nil, err
	}
	if mount {
		fsdir, found, err := req.Option(ipfsMountKwd).String()
		if err != nil {
			return nil, err
		}
		if !found {
			fsdir = cfg.Mounts.IPFS
		}

		nsdir, found, err := req.Option(ipnsMountKwd).String()
		if err != nil {
			return nil, err
		}
		if !found {
			nsdir = cfg.Mounts.IPNS
		}

		err = commands.Mount(node, fsdir, nsdir)
		if err != nil {
			return nil, err
		}
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
