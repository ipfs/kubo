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

var cmdIpfsCat = &commander.Command{
	UsageLine: "cat",
	Short:     "Show ipfs object data.",
	Long: `ipfs cat <ipfs-path> - Show ipfs object data.

    Retrieves the object named by <ipfs-path> and displays the Data
    it contains.
`,
	Run:  catCmd,
	Flag: *flag.NewFlagSet("ipfs-cat", flag.ExitOnError),
}

func catCmd(c *commander.Command, inp []string) error {
	if len(inp) < 1 {
		u.POut(c.Long)
		return nil
	}

	com := daemon.NewCommand()
	com.Command = "cat"
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

		return commands.Cat(n, com.Args, com.Opts, os.Stdout)
	}
	return nil
}
