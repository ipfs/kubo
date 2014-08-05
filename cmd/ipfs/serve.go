package main

import (
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

func serveCmd(c *commander.Command, _ []string) error {
	n, err := localNode()
	if err != nil {
		return err
	}

	return h.Serve("127.0.0.1", 80, n)
}
