package main

import (
	"os"

	"github.com/jbenet/go-ipfs/core/commands"
	"github.com/jbenet/go-ipfs/daemon"
	u "github.com/jbenet/go-ipfs/util"
	"github.com/spf13/cobra"
)

var cmdIpfsLs = &cobra.Command{
	Use:   "ls",
	Short: "List links from an object.",
	Long: `ipfs ls <ipfs-path> - List links from an object.

    Retrieves the object named by <ipfs-path> and displays the links
    it contains, with the following format:

    <link base58 hash> <link size in bytes> <link name>

`,
	Run: lsCmd,
}

func init() {
	CmdIpfs.AddCommand(cmdIpfsLs)
}

func lsCmd(c *cobra.Command, inp []string) {
	if len(inp) < 1 {
		u.POut(c.Long)
		return
	}

	com := daemon.NewCommand()
	com.Command = "ls"
	com.Args = inp
	err := daemon.SendCommand(com, "localhost:12345")
	if err != nil {
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

		err = commands.Ls(n, com.Args, com.Opts, os.Stdout)
		if err != nil {
			u.PErr(err.Error())
			return
		}
	}
}
