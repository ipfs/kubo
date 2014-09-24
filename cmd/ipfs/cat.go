package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/jbenet/go-ipfs/core/commands"
	"github.com/jbenet/go-ipfs/daemon"
	u "github.com/jbenet/go-ipfs/util"
)

var cmdIpfsCat = &cobra.Command{
	Use: "cat",
	Short:     "Show ipfs object data.",
	Long: `ipfs cat <ipfs-path> - Show ipfs object data.

    Retrieves the object named by <ipfs-path> and displays the Data
    it contains.
`,
	Run:  catCmd,
}

func init() {
	CmdIpfs.AddCommand(cmdIpfsCat)
}

func catCmd(c *cobra.Command, inp []string) {
	if len(inp) < 1 {
		u.POut(c.Long)
		return
	}

	com := daemon.NewCommand()
	com.Command = "cat"
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

		err = commands.Cat(n, com.Args, com.Opts, os.Stdout)
		if err != nil {
			u.PErr(err.Error())
			return
		}
	}
}
