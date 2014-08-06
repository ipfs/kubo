package main

import (
	"errors"
	"fmt"
	"github.com/gonuts/flag"
	"github.com/jbenet/commander"
	h "github.com/jbenet/go-ipfs/http"
	"strconv"
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
	cmdIpfsServe.Flag.String("hostname", "localhost", "Hostname")
}

func serveCmd(c *commander.Command, _ []string) error {
	port := c.Flag.Lookup("port").Value.Get().(uint)
	if port < 1 || port > 65535 {
		return errors.New("invalid port number")
	}

	hostname := c.Flag.Lookup("hostname").Value.Get().(string)

	n, err := localNode()
	if err != nil {
		return err
	}

	address := hostname + ":" + strconv.FormatUint(uint64(port), 10)
	fmt.Printf("Serving on %s\n", address)

	return h.Serve(address, n)
}
