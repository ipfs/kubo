package main

import (
	"github.com/jbenet/commander"
	u "github.com/jbenet/go-ipfs/util"
)

// The IPFS version.
const Version = "0.1.0"

var cmdIpfsVersion = &commander.Command{
	UsageLine: "version",
	Short:     "Show ipfs version information.",
	Long: `ipfs version - Show ipfs version information.

    Returns the current version of ipfs and exits.
  `,
	Run: versionCmd,
}

func init() {
	cmdIpfsVersion.Flag.Bool("number", false, "show only the number")
}

func versionCmd(c *commander.Command, _ []string) error {
	number := c.Flag.Lookup("number").Value.Get().(bool)
	if !number {
		u.POut("ipfs version ")
	}
	u.POut("%s\n", Version)
	return nil
}
