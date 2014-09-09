package main

import (
	"fmt"
	"os"

	"github.com/gonuts/flag"
	"github.com/jbenet/commander"
	daemon "github.com/jbenet/go-ipfs/daemon"
	u "github.com/jbenet/go-ipfs/util"
)

var cmdIpfsAdd = &commander.Command{
	UsageLine: "add",
	Short:     "Add an object to ipfs.",
	Long: `ipfs add <path>... - Add objects to ipfs.

    Adds contents of <path> to ipfs. Use -r to add directories.
    Note that directories are added recursively, to form the ipfs
    MerkleDAG. A smarter partial add with a staging area (like git)
    remains to be implemented.
`,
	Run:  addCmd,
	Flag: *flag.NewFlagSet("ipfs-add", flag.ExitOnError),
}

func init() {
	cmdIpfsAdd.Flag.Bool("r", false, "add objects recursively")
}

func addCmd(c *commander.Command, inp []string) error {
	if len(inp) < 1 {
		u.POut(c.Long)
		return nil
	}

	cmd := daemon.NewCommand()
	cmd.Command = "add"
	fmt.Println(inp)
	cmd.Args = inp
	cmd.Opts["r"] = c.Flag.Lookup("r").Value.Get()
	err := daemon.SendCommand(cmd, "localhost:12345")
	if err != nil {
		// Do locally
		n, err := localNode(false)
		if err != nil {
			return err
		}

		daemon.ExecuteCommand(cmd, n, os.Stdout)
	}
	return nil
}
