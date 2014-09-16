package main

import (
	"errors"
	"fmt"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/daemon"
	h "github.com/jbenet/go-ipfs/server/http"
)

var cmdIpfsServe = &commander.Command{
	UsageLine: "serve",
	Short:     "Serve an interface to ipfs",
	Subcommands: []*commander.Command{
		cmdIpfsServeHttp,
	},
	Flag: *flag.NewFlagSet("ipfs-serve", flag.ExitOnError),
}

var cmdIpfsServeHttp = &commander.Command{
	UsageLine: "http",
	Short:     "Serve an HTTP API",
	Long:      `ipfs serve http - Serve an http gateway into ipfs.`,
	Run:       serveHttpCmd,
	Flag:      *flag.NewFlagSet("ipfs-serve-http", flag.ExitOnError),
}

func init() {
	cmdIpfsServeHttp.Flag.Uint("port", 8080, "Port number")
	cmdIpfsServeHttp.Flag.String("hostname", "localhost", "Hostname")
}

func serveHttpCmd(c *commander.Command, _ []string) error {
	port := c.Flag.Lookup("port").Value.Get().(uint)
	if port < 1 || port > 65535 {
		return errors.New("invalid port number")
	}

	n, err := localNode(true)
	if err != nil {
		return err
	}

	fmt.Println("starting new daemon listener...")
	dl, err := daemon.NewDaemonListener(n, "localhost:12345")
	if err != nil {
		return err
	}
	go dl.Listen()
	defer dl.Close()

	hostname := c.Flag.Lookup("hostname").Value.Get().(string)
	address := fmt.Sprintf("%s:%d", hostname, port)
	fmt.Printf("Serving on %s\n", address)

	return h.Serve(address, n)
}
