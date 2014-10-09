package main

import (
	"fmt"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	h "github.com/jbenet/go-ipfs/server/http"
)

var cmdIpfsServe = &commander.Command{
	UsageLine: "serve",
	Short:     "Serve an interface to ipfs",
	Subcommands: []*commander.Command{
		cmdIpfsServeHTTP,
	},
	Flag: *flag.NewFlagSet("ipfs-serve", flag.ExitOnError),
}

var cmdIpfsServeHTTP = &commander.Command{
	UsageLine: "http",
	Short:     "Serve an HTTP API",
	Long:      `ipfs serve http - Serve an http gateway into ipfs.`,
	Run:       serveHTTPCmd,
	Flag:      *flag.NewFlagSet("ipfs-serve-http", flag.ExitOnError),
}

func init() {
	cmdIpfsServeHTTP.Flag.String("address", "/ip4/127.0.0.1/tcp/8080", "Listen Address")
}

func serveHTTPCmd(c *commander.Command, _ []string) error {
	cc, err := setupCmdContext(c, true)
	if err != nil {
		return err
	}
	defer cc.daemon.Close()

	address := c.Flag.Lookup("address").Value.Get().(string)
	maddr, err := ma.NewMultiaddr(address)
	if err != nil {
		return err
	}

	fmt.Printf("Serving on %s\n", address)
	return h.Serve(maddr, cc.node)
}
