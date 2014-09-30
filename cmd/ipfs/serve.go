package main

import (
	"errors"
	"fmt"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

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
	cmdIpfsServeHttp.Flag.String("address", "/ip4/127.0.0.1/tcp/8080", "Listen Address")
}

func serveHttpCmd(c *commander.Command, _ []string) error {
	conf, err := getConfigDir(c.Parent.Parent)
	if err != nil {
		return err
	}

	n, err := localNode(conf, true)
	if err != nil {
		return err
	}

	// launch the API RPC endpoint.
	if n.Config.Addresses.API == "" {
		return errors.New("no config.RPCAddress endpoint supplied")
	}

	maddr, err := ma.NewMultiaddr(n.Config.Addresses.API)
	if err != nil {
		return err
	}

	dl, err := daemon.NewDaemonListener(n, maddr)
	if err != nil {
		fmt.Println("Failed to create daemon listener.")
		return err
	}
	go dl.Listen()
	defer dl.Close()

	address := c.Flag.Lookup("address").Value.Get().(string)
	maddr, err = ma.NewMultiaddr(address)
	if err != nil {
		return err
	}

	fmt.Printf("Serving on %s\n", address)
	return h.Serve(maddr, n)
}
