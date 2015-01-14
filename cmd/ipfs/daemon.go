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

	originEnvKey = "API_ORIGIN"

	webuiPath = "/ipfs/QmTWvqK9dYvqjAMAcCeUun8b45Fwu7wPhEN9B9TsGbkXfJ"
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

	// acquire the repo lock _before_ constructing a node. we need to make
	// sure we are permitted to access the resources (datastore, etc.)
	repo := fsrepo.At(req.Context().ConfigRoot)
	if err := repo.Open(); err != nil {
		return nil, debugerror.Errorf("Couldn't obtain lock. Is another daemon already running?")
	}
	defer repo.Close()

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

	if gatewayMaddr != nil {
		go func() {
			err := listenAndServeGateway(node, gatewayMaddr)
			if err != nil {
				log.Error(err)
			}
		}()
	}

	return nil, listenAndServeAPI(node, req, apiMaddr)
}

func listenAndServeAPI(node *core.IpfsNode, req cmds.Request, addr ma.Multiaddr) error {
	origin := os.Getenv(originEnvKey)
	cmdHandler := cmdsHttp.NewHandler(*req.Context(), commands.Root, origin)
	gateway, err := NewGatewayHandler(node)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle(cmdsHttp.ApiPath+"/", cmdHandler)
	mux.Handle("/ipfs/", gateway)
	mux.Handle("/webui/", &redirectHandler{webuiPath})
	return listenAndServe("API", node, addr, mux)
}

// the gateway also listens on its own address:port in addition to the API listener
func listenAndServeGateway(node *core.IpfsNode, addr ma.Multiaddr) error {
	gateway, err := NewGatewayHandler(node)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/ipfs/", gateway)
	return listenAndServe("gateway", node, addr, mux)
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
