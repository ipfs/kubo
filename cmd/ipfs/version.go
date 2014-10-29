package main

import (
	config "github.com/jbenet/go-ipfs/config"
	u "github.com/jbenet/go-ipfs/util"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
)

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
	u.POut("%s\n", config.CurrentVersionNumber)
	return nil
}
