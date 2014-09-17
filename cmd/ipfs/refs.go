package main

import (
	"os"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	commands "github.com/jbenet/go-ipfs/core/commands"
	"github.com/jbenet/go-ipfs/daemon"
	u "github.com/jbenet/go-ipfs/util"
)

var cmdIpfsRefs = &commander.Command{
	UsageLine: "refs",
	Short:     "List link hashes from an object.",
	Long: `ipfs refs <ipfs-path> - List link hashes from an object..

    Retrieves the object named by <ipfs-path> and displays the link
    hashes it contains, with the following format:

    <link base58 hash>

    Note: list all refs recursively with -r.

`,
	Run:  refCmd,
	Flag: *flag.NewFlagSet("ipfs-refs", flag.ExitOnError),
}

func init() {
	cmdIpfsRefs.Flag.Bool("r", false, "recursive: list refs recursively")
	cmdIpfsRefs.Flag.Bool("u", false, "unique: list each ref only once")
}

func refCmd(c *commander.Command, inp []string) error {
	if len(inp) < 1 {
		u.POut(c.Long)
		return nil
	}

	cmd := daemon.NewCommand()
	cmd.Command = "refs"
	cmd.Args = inp
	cmd.Opts["r"] = c.Flag.Lookup("r").Value.Get()
	cmd.Opts["u"] = c.Flag.Lookup("u").Value.Get()
	err := daemon.SendCommand(cmd, "localhost:12345")
	if err != nil {
		// Do locally
		conf, err := getConfigDir(c.Parent)
		if err != nil {
			return err
		}
		n, err := localNode(conf, false)
		if err != nil {
			return err
		}

		return commands.Refs(n, cmd.Args, cmd.Opts, os.Stdout)
	}
	return nil
}
