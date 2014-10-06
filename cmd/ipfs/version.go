package main

import (
	semver "github.com/coreos/go-semver/semver"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
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

var currentVersion *semver.Version

func init() {
	currentVersion, _ = semver.NewVersion(Version)
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
