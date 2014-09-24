package main

import (
	u "github.com/jbenet/go-ipfs/util"
	"github.com/spf13/cobra"
)

// The IPFS version.
const Version = "0.1.0"

var cmdIpfsVersion = &cobra.Command{
	Use:   "version",
	Short: "Show ipfs version information.",
	Long: `ipfs version - Show ipfs version information.

    Returns the current version of ipfs and exits.
  `,
	Run: versionCmd,
}

var number bool

func init() {
	cmdIpfsVersion.Flags().BoolVarP(&number, "number", "n", false, "show only the number")
	CmdIpfs.AddCommand(cmdIpfsVersion)
}

func versionCmd(c *cobra.Command, _ []string) {
	if !number {
		u.POut("ipfs version ")
	}
	u.POut("%s\n", Version)
}
