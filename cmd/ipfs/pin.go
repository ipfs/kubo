package main

import (
	"os"

	"github.com/gonuts/flag"
	"github.com/jbenet/commander"
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

	com := daemon.NewCommand()
	com.Command = "pin"
	com.Args = inp

	err := daemon.SendCommand(com, "localhost:12345")
	if err != nil {
		n, err := localNode(false)
		if err != nil {
			return err
		}

		daemon.ExecuteCommand(com, n, os.Stdout)
	}
	return nil
}
