package main

import (
	"os"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
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
	err := daemon.SendCommand(com, "localhost:12345")
	if err != nil {
		conf := getConfig(c.Parent)
		n, err := localNode(conf, false)
		if err != nil {
			return err
		}

		return commands.Ls(n, com.Args, com.Opts, os.Stdout)
	}

	return nil
}
