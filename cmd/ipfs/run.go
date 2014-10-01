package main

import (
	"errors"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	"github.com/jbenet/go-ipfs/daemon"
)

var cmdIpfsRun = &commander.Command{
	UsageLine: "run",
	Short:     "run local ifps node.",
	Long: `run a local ipfs node with no other interface.
`,
	Run:  runCmd,
	Flag: *flag.NewFlagSet("ipfs-run", flag.ExitOnError),
}

func runCmd(c *commander.Command, inp []string) error {
	conf, err := getConfigDir(c.Parent)
	if err != nil {
		return err
	}
	n, err := localNode(conf, true)
	if err != nil {
		return err
	}

	// launch the RPC endpoint.
	if n.Config.Addresses.API == "" {
		return errors.New("no config.Addresses.API endpoint supplied")
	}

	maddr, err := ma.NewMultiaddr(n.Config.Addresses.API)
	if err != nil {
		return err
	}

	dl, err := daemon.NewDaemonListener(n, maddr, conf)
	if err != nil {
		return err
	}
	dl.Listen()
	dl.Close()
	return nil
}
