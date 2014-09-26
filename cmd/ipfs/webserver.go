package main

import (
	"net/http"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"

	"github.com/jbenet/go-ipfs/daemon"
	"github.com/jbenet/go-ipfs/webserver"
	u   "github.com/jbenet/go-ipfs/util"
)

var cmdIpfsWebserver = &commander.Command{
	UsageLine: "webserver",
	Short:     "Start a webserver for IPFS.",
	Long: `ipfs webserver <listen-expr> - Start a webserver listening on <listen-expr>.

    Serve all ipfs objects over HTTP. The HTTP server will be listening
    on <listen-expr> which can be ":8080" for example. HTTP server is
    read-only for the moment, but it is planned to make it read-write.
    Access control should be implemented by a reverse proxy server.

`,
	Run:  webserverCmd,
	Flag: *flag.NewFlagSet("ipfs-webserver", flag.ExitOnError),
}

func webserverCmd(c *commander.Command, inp []string) error {
	if len(inp) < 1 || len(inp[0]) == 0 {
		u.POut(c.Long)
		return nil
	}

	// Get configuration file
	conf, err := getConfigDir(c.Parent)
	if err != nil {
		return err
	}

	// Load IPFS node from config
	n, err := localNode(conf, true)
	if err != nil {
		return err
	}

	dl, err := daemon.NewRPCDaemonListener(n)
	if err != nil {
		return err
	}
	go dl.Listen()
	defer dl.Close()

	listenExpr := inp[0];
	server := webserver.NewWebServer(n);

	return http.ListenAndServe(listenExpr, server)
}
