package main

import (
	"os"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	"github.com/jbenet/go-ipfs/config"
	"github.com/jbenet/go-ipfs/core/commands"
	"github.com/jbenet/go-ipfs/daemon"
	u "github.com/jbenet/go-ipfs/util"
)

var cmdIpfsLs = &commander.Command{
	UsageLine: "ls",
	Short:     "List links from an object.",
	Long: `ipfs ls <ipfs-path> - List links from an object.

    Retrieves the object named by <ipfs-path> and displays the links
    it contains, with the following format:

    <link base58 hash> <link size in bytes> <link name>

`,
	Run:  lsCmd,
	Flag: *flag.NewFlagSet("ipfs-ls", flag.ExitOnError),
}

func lsCmd(c *commander.Command, inp []string) error {
	if len(inp) < 1 {
		u.POut(c.Long)
		return nil
	}

	com := daemon.NewCommand()
	com.Command = "ls"
	com.Args = inp

	confDir, err := getConfigDir(c.Parent)
	if err != nil {
		return err
	}
	conf, err := config.Load(confDir + "/config")
	dAddr, err := ma.NewMultiaddr(conf.RPCAddress)
	if err != nil {
		return err
	}

	err = daemon.SendCommand(com, dAddr)
	if err != nil {
		// Do locally
		n, err := localNode(confDir, false)
		if err != nil {
			return err
		}

		return commands.Ls(n, com.Args, com.Opts, os.Stdout)
	}

	return nil
}
