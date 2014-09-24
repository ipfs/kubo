package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/jbenet/go-ipfs/core/commands"
	"github.com/jbenet/go-ipfs/daemon"
	u "github.com/jbenet/go-ipfs/util"
)

// Error indicating the max depth has been exceded.
var ErrDepthLimitExceeded = fmt.Errorf("depth limit exceeded")

var cmdIpfsAdd = &cobra.Command{
	Use: "add",
	Short:     "Add an object to ipfs.",
	Long: `ipfs add <path>... - Add objects to ipfs.

    Adds contents of <path> to ipfs. Use -r to add directories.
    Note that directories are added recursively, to form the ipfs
    MerkleDAG. A smarter partial add with a staging area (like git)
    remains to be implemented.
`,
	Run:  addCmd,
}

var recursive bool
func init() {
	cmdIpfsAdd.Flags().BoolVarP(&recursive, "recursive", "r", false, "add objects recursively")
	CmdIpfs.AddCommand(cmdIpfsAdd)
}

func addCmd(c *cobra.Command, inp []string) {
	if len(inp) < 1 {
		u.POut(c.Long)
		return
	}

	cmd := daemon.NewCommand()
	cmd.Command = "add"
	cmd.Args = inp
	cmd.Opts["r"] = recursive
	err := daemon.SendCommand(cmd, "localhost:12345")
	if err != nil {
		// Do locally
		conf, err := getConfigDir(c)
		if err != nil {
			u.PErr(err.Error())
			return
		}
		n, err := localNode(conf, false)
		if err != nil {
			u.PErr(err.Error())
			return
		}

		err = commands.Add(n, cmd.Args, cmd.Opts, os.Stdout)
		if err != nil {
			u.PErr(err.Error())
			return
		}
	}
}
