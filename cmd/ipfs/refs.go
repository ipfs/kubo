package main

import (
	"os"

	commands "github.com/jbenet/go-ipfs/core/commands"
	"github.com/jbenet/go-ipfs/daemon"
	u "github.com/jbenet/go-ipfs/util"
	"github.com/spf13/cobra"
)

var cmdIpfsRefs = &cobra.Command{
	Use:   "refs",
	Short: "List link hashes from an object.",
	Long: `ipfs refs <ipfs-path> - List link hashes from an object..

    Retrieves the object named by <ipfs-path> and displays the link
    hashes it contains, with the following format:

    <link base58 hash>

    Note: list all refs recursively with -r.

`,
	Run: refCmd,
}

var (
	refsRecursive bool
	unique        bool
)

func init() {
	cmdIpfsRefs.Flags().BoolVarP(&refsRecursive, "recursive", "r", false, "recursive: list refs recursively")
	cmdIpfsRefs.Flags().BoolVarP(&unique, "unique", "u", false, "unique: list each ref only once")
	CmdIpfs.AddCommand(cmdIpfsRefs)
}

func refCmd(c *cobra.Command, inp []string) {
	if len(inp) < 1 {
		u.POut(c.Long)
		return
	}

	cmd := daemon.NewCommand()
	cmd.Command = "refs"
	cmd.Args = inp
	cmd.Opts["r"] = refsRecursive
	cmd.Opts["u"] = unique
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

		err = commands.Refs(n, cmd.Args, cmd.Opts, os.Stdout)
		if err != nil {
			u.PErr(err.Error())
			return
		}
	}
}
