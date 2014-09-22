package main

import (
	"os"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/core/commands"
	"github.com/jbenet/go-ipfs/daemon"
	u "github.com/jbenet/go-ipfs/util"
)

var cmdIpfsPin = &commander.Command{
	UsageLine: "pin",
	Short:     "pin an ipfs object to local storage.",
	Long: `ipfs pin <ipfs-path> - pin ipfs object to local storage.

    Retrieves the object named by <ipfs-path> and stores it locally
    on disk.
`,
	Run:  pinCmd,
	Flag: *flag.NewFlagSet("ipfs-pin", flag.ExitOnError),
}

func pinCmd(c *commander.Command, inp []string) error {
	if len(inp) < 1 {
		u.POut(c.Long)
		return nil
	}

	cmd := daemon.NewCommand()
	cmd.Command = "pin"
	cmd.Args = inp

	err := daemon.SendCommand(cmd, "localhost:12345")
	if err != nil {
		conf, err := getConfigDir(c.Parent)
		if err != nil {
			return err
		}
		n, err := localNode(conf, false)
		if err != nil {
			return err
		}

		return commands.Pin(n, cmd.Args, cmd.Opts, os.Stdout)
	}
	return nil
}
