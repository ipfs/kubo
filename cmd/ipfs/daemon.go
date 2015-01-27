package main

import (
	"fmt"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	cmds "github.com/jbenet/go-ipfs/commands"
	commands "github.com/jbenet/go-ipfs/core/commands"
	corehttp "github.com/jbenet/go-ipfs/core/corehttp"
	fsrepo "github.com/jbenet/go-ipfs/repo/fsrepo"
	util "github.com/jbenet/go-ipfs/util"
	"github.com/jbenet/go-ipfs/util/debugerror"
)

const (
	initOptionKwd = "init"
	mountKwd      = "mount"
	ipfsMountKwd  = "mount-ipfs"
	ipnsMountKwd  = "mount-ipns"
	// apiAddrKwd    = "address-api"
	// swarmAddrKwd  = "address-swarm"
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

		// TODO: add way to override addresses. tricky part: updating the config if also --init.
		// cmds.StringOption(apiAddrKwd, "Address for the daemon rpc API (overrides config)"),
		// cmds.StringOption(swarmAddrKwd, "Address for the swarm socket (overrides config)"),
	},
	Subcommands: map[string]*cmds.Command{},
	Run:         daemonFunc,
}

func daemonFunc(req cmds.Request, res cmds.Response) {

	fmt.Println("Initializing daemon...")
	// first, whether user has provided the initialization flag. we may be
	// running in an uninitialized state.
	initialize, _, err := req.Option(initOptionKwd).Bool()
	if err != nil {
		res.SetError(err, cmds.ErrNormal)
		return
	}
	if initialize {

		// now, FileExists is our best method of detecting whether IPFS is
		// configured. Consider moving this into a config helper method
		// `IsInitialized` where the quality of the signal can be improved over
		// time, and many call-sites can benefit.
		if !util.FileExists(req.Context().ConfigRoot) {
			err := initWithDefaults(req.Context().ConfigRoot)
			if err != nil {
				res.SetError(debugerror.Wrap(err), cmds.ErrNormal)
				return
			}
		}
	}

	// To ensure that IPFS has been initialized, fetch the config. Do this
	// _before_ acquiring the daemon lock so the user gets an appropriate error
	// message.
	// NB: It's safe to read the config without the daemon lock, but not safe
	// to write.
	ctx := req.Context()
	cfg, err := ctx.GetConfig()
	if err != nil {
		res.SetError(err, cmds.ErrNormal)
		return
	}

	// acquire the repo lock _before_ constructing a node. we need to make
	// sure we are permitted to access the resources (datastore, etc.)
	repo := fsrepo.At(req.Context().ConfigRoot)
	if err := repo.Open(); err != nil {
		res.SetError(debugerror.Errorf("Couldn't obtain lock. Is another daemon already running?"), cmds.ErrNormal)
		return
	}
	defer repo.Close()

	// OK!!! Now we're ready to construct the node.
	// make sure we construct an online node.
	ctx.Online = true
	node, err := ctx.GetNode()
	if err != nil {
		res.SetError(err, cmds.ErrNormal)
		return
	}

	// verify api address is valid multiaddr
	apiMaddr, err := ma.NewMultiaddr(cfg.Addresses.API)
	if err != nil {
		res.SetError(err, cmds.ErrNormal)
		return
	}

	var gatewayMaddr ma.Multiaddr
	if len(cfg.Addresses.Gateway) > 0 {
		// ignore error for gateway address
		// if there is an error (invalid address), then don't run the gateway
		gatewayMaddr, _ = ma.NewMultiaddr(cfg.Addresses.Gateway)
		if gatewayMaddr == nil {
			log.Errorf("Invalid gateway address: %s", cfg.Addresses.Gateway)
		}
	}

	// mount if the user provided the --mount flag
	mount, _, err := req.Option(mountKwd).Bool()
	if err != nil {
		res.SetError(err, cmds.ErrNormal)
		return
	}
	if mount {
		fsdir, found, err := req.Option(ipfsMountKwd).String()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		if !found {
			fsdir = cfg.Mounts.IPFS
		}

		nsdir, found, err := req.Option(ipnsMountKwd).String()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		if !found {
			nsdir = cfg.Mounts.IPNS
		}

		err = commands.Mount(node, fsdir, nsdir)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		fmt.Printf("IPFS mounted at: %s\n", fsdir)
		fmt.Printf("IPNS mounted at: %s\n", nsdir)
	}

	if gatewayMaddr != nil {
		go func() {
			err := corehttp.ListenAndServe(node, gatewayMaddr.String(), corehttp.GatewayOption)
			if err != nil {
				log.Error(err)
			}
		}()
	}

	var opts = []corehttp.ServeOption{
		corehttp.CommandsOption(*req.Context()),
		corehttp.WebUIOption,
		corehttp.GatewayOption,
	}
	if err := corehttp.ListenAndServe(node, apiMaddr.String(), opts...); err != nil {
		res.SetError(err, cmds.ErrNormal)
		return
	}
}
