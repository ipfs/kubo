package main

import (
	"errors"
	"github.com/gonuts/flag"
	"github.com/jbenet/commander"
	h "github.com/jbenet/go-ipfs/http"
)

var cmdIpfsServe = &commander.Command{
	UsageLine: "serve",
	Short:     "Serve an HTTP API",
	Long:      `ipfs serve - Serve an http gateway into ipfs.`,
	Run:       serveCmd,
	Flag:      *flag.NewFlagSet("ipfs-serve", flag.ExitOnError),
}

func init() {
	cmdIpfsServe.Flag.Uint("port", 80, "Port number")
}

func serveCmd(c *commander.Command, _ []string) error {
	port := c.Flag.Lookup("port").Value.Get().(uint)
	if port < 1 || port > 65535 {
		return errors.New("invalid port number")
	}

	n, err := localNode()
	if err != nil {
		return err
	}

	return h.Serve("127.0.0.1", port, n)
}
