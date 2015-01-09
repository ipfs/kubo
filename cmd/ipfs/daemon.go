package main

import (
	"fmt"
	"net/http"
	"os"

	manners "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/braintree/manners"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
	cmds "github.com/jbenet/go-ipfs/commands"
	cmdsHttp "github.com/jbenet/go-ipfs/commands/http"
	core "github.com/jbenet/go-ipfs/core"
	commands "github.com/jbenet/go-ipfs/core/commands"
	daemon "github.com/jbenet/go-ipfs/core/daemon"
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
	originEnvKey = "API_ORIGIN"
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

func daemonFunc(req cmds.Request) (interface{}, error) {

	// first, whether user has provided the initialization flag. we may be
	// running in an uninitialized state.
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

	// To ensure that IPFS has been initialized, fetch the config. Do this
	// _before_ acquiring the daemon lock so the user gets an appropriate error
	// message.
	// NB: It's safe to read the config without the daemon lock, but not safe
	// to write.
	ctx := req.Context()
	cfg, err := ctx.GetConfig()
	if err != nil {
		return nil, err
	}

	// acquire the daemon lock _before_ constructing a node. we need to make
	// sure we are permitted to access the resources (datastore, etc.)
	lock, err := daemon.Lock(req.Context().ConfigRoot)
	if err != nil {
		return nil, debugerror.Errorf("Couldn't obtain lock. Is another daemon already running?")
	}
	defer lock.Close()

	// OK!!! Now we're ready to construct the node.
	// make sure we construct an online node.
	ctx.Online = true
	node, err := ctx.GetNode()
	if err != nil {
		return nil, err
	}

	// verify api address is valid multiaddr
	apiMaddr, err := ma.NewMultiaddr(cfg.Addresses.API)
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
		fmt.Printf("IPFS mounted at: %s\n", fsdir)
		fmt.Printf("IPNS mounted at: %s\n", nsdir)
	}

	return nil, listenAndServeAPI(node, req, apiMaddr)
}

func listenAndServeAPI(node *core.IpfsNode, req cmds.Request, addr ma.Multiaddr) error {

	_, host, err := manet.DialArgs(addr)
	if err != nil {
		return err
	}

	origin := os.Getenv(originEnvKey)

	server := manners.NewServer()
	mux := http.NewServeMux()
	cmdHandler := cmdsHttp.NewHandler(*req.Context(), commands.Root, origin)
	mux.Handle(cmdsHttp.ApiPath+"/", cmdHandler)

	ifpsHandler := &ipfsHandler{node: node}
	ifpsHandler.LoadTemplate()

	mux.Handle("/ipfs/", ifpsHandler)

	// if the server exits beforehand
	var serverError error
	serverExited := make(chan struct{})

	go func() {
		fmt.Printf("daemon listening on %s\n", addr)
		serverError = server.ListenAndServe(host, mux)
		close(serverExited)
	}()

	// wait for server to exit.
	select {
	case <-serverExited:

	// if node being closed before server exits, close server
	case <-node.Closing():
		log.Infof("daemon at %s terminating...", addr)
		server.Shutdown <- true
		<-serverExited // now, DO wait until server exits
	}

	log.Infof("daemon at %s terminated", addr)
	return serverError
}
